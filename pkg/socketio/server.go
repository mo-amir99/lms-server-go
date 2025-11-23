package socketio

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	socket "github.com/zishang520/socket.io/socket"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
	jwtutil "github.com/mo-amir99/lms-server-go/internal/utils/jwt"
	"github.com/mo-amir99/lms-server-go/pkg/streamcache"
)

// StreamingLimits defines production-ready streaming constraints.
type StreamingLimits struct {
	MaxConcurrentStreamsPerUser int
	MaxViewersPerStream         int
	MaxTotalConcurrentStreams   int
	MaxStreamDuration           time.Duration
	StreamStartCooldown         time.Duration
}

type userStreamActivity struct {
	lastStreamStart time.Time
	activeStreams   int
}

// Server wraps the Socket.IO server with streaming functionality.
type Server struct {
	io          *socket.Server
	db          *gorm.DB
	logger      *slog.Logger
	streamCache *streamcache.Cache
	limits      StreamingLimits
	jwtSecret   string

	heartbeatStop chan struct{}
	heartbeatWG   sync.WaitGroup

	connMutex   sync.RWMutex
	connections map[string]*socket.Socket

	activityMu   sync.Mutex
	userActivity map[string]*userStreamActivity
}

// NewServer creates a new Socket.IO server with streaming support.
func NewServer(db *gorm.DB, logger *slog.Logger, streamCache *streamcache.Cache, jwtSecret string) (*Server, error) {
	opts := socket.DefaultServerOptions()
	opts.SetPingTimeout(60 * time.Second)
	opts.SetPingInterval(25 * time.Second)
	opts.SetServeClient(false)
	opts.SetPath("/socket.io")

	server := socket.NewServer(nil, opts)

	s := &Server{
		io:          server,
		db:          db,
		logger:      logger,
		streamCache: streamCache,
		jwtSecret:   jwtSecret,
		limits: StreamingLimits{
			MaxConcurrentStreamsPerUser: 1,
			MaxViewersPerStream:         100,
			MaxTotalConcurrentStreams:   50,
			MaxStreamDuration:           4 * time.Hour,
			StreamStartCooldown:         30 * time.Second,
		},
		connections:  make(map[string]*socket.Socket),
		userActivity: make(map[string]*userStreamActivity),
	}

	s.setupEventHandlers()
	s.startHeartbeat()

	return s, nil
}

// GetHandler returns the HTTP handler for Socket.IO.
func (s *Server) GetHandler() http.Handler {
	return s.io.ServeHandler(nil)
}

// Close shuts down the Socket.IO server.
func (s *Server) Close() error {
	if stop := s.heartbeatStop; stop != nil {
		close(stop)
		s.heartbeatWG.Wait()
		s.heartbeatStop = nil
	}

	done := make(chan struct{})
	s.io.Close(func() {
		close(done)
	})

	<-done
	return nil
}

// setupEventHandlers configures all Socket.IO event handlers.
func (s *Server) setupEventHandlers() {
	s.io.Use(s.connectionMiddleware)
	s.io.On("connection", func(args ...any) {
		sock, ok := args[0].(*socket.Socket)
		if !ok {
			s.logger.Error("unexpected connection payload", slog.Any("payload", args))
			return
		}
		s.handleConnection(sock)
	})
}

func (s *Server) connectionMiddleware(sock *socket.Socket, next func(*socket.ExtendedError)) {
	token := s.extractToken(sock)
	if token == "" {
		s.logger.Warn("socket connection rejected: missing token")
		next(socket.NewExtendedError("missing authentication token", map[string]any{"code": "MISSING_TOKEN"}))
		return
	}

	claims, err := jwtutil.VerifyToken(token, s.jwtSecret)
	if err != nil {
		s.logger.Warn("socket connection rejected: invalid token", slog.String("error", err.Error()))
		next(socket.NewExtendedError("invalid token", map[string]any{"code": "INVALID_TOKEN"}))
		return
	}

	var userData user.User
	if err := s.db.Preload("Subscription").First(&userData, "id = ?", claims.UserID).Error; err != nil {
		s.logger.Warn("socket connection rejected: user not found", slog.Any("userId", claims.UserID), slog.String("error", err.Error()))
		next(socket.NewExtendedError("user not found", map[string]any{"code": "USER_NOT_FOUND"}))
		return
	}

	sock.SetData(&userData)
	next(nil)
}

func (s *Server) handleConnection(sock *socket.Socket) {
	userData := s.getUserFromSocket(sock)
	if userData == nil {
		s.logger.Error("connection established without user context")
		sock.Disconnect(true)
		return
	}

	s.connMutex.Lock()
	s.connections[s.socketID(sock)] = sock
	s.connMutex.Unlock()

	s.logger.Info("WebSocket connected",
		slog.String("user", userData.FullName),
		slog.String("userId", userData.ID.String()),
		slog.String("connId", string(sock.Id())),
	)

	confirmData := map[string]any{
		"userId":    userData.ID.String(),
		"userName":  userData.FullName,
		"userEmail": userData.Email,
		"userType":  userData.UserType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if userData.Subscription != nil {
		displayName := "Subscription"
		if userData.Subscription.DisplayName != nil {
			displayName = *userData.Subscription.DisplayName
		}

		confirmData["subscription"] = map[string]any{
			"id":              userData.Subscription.ID.String(),
			"displayName":     displayName,
			"identifierName":  userData.Subscription.IdentifierName,
			"isActive":        userData.Subscription.Active,
			"subscriptionEnd": userData.Subscription.SubscriptionEnd.Format(time.RFC3339),
		}
	}

	if err := sock.Emit("connectionConfirmed", confirmData); err != nil {
		s.logger.Warn("failed to emit connection confirmation", slog.String("error", err.Error()))
	}

	sock.Join(userRoom(userData.ID.String()))
	s.registerEventHandlers(sock)
}

func (s *Server) registerEventHandlers(sock *socket.Socket) {
	sock.On("getActiveStreams", func(args ...any) {
		s.handleGetActiveStreams(sock)
	})

	sock.On("startStream", func(args ...any) {
		payload := mapArg(args)
		if payload == nil {
			s.emitError(sock, "INVALID_INPUT", "stream payload is required")
			return
		}
		s.handleStartStream(sock, payload)
	})

	sock.On("joinStream", func(args ...any) {
		streamID := stringArg(args)
		if streamID == "" {
			s.emitError(sock, "INVALID_INPUT", "stream ID is required")
			return
		}
		s.handleJoinStream(sock, streamID)
	})

	sock.On("leaveStream", func(args ...any) {
		streamID := stringArg(args)
		if streamID == "" {
			s.emitError(sock, "INVALID_INPUT", "stream ID is required")
			return
		}
		s.handleLeaveStream(sock, streamID, "client-request")
	})

	sock.On("endStream", func(args ...any) {
		streamID := stringArg(args)
		if streamID == "" {
			s.emitError(sock, "INVALID_INPUT", "stream ID is required")
			return
		}
		s.handleEndStream(sock, streamID)
	})

	sock.On("updateStreamMedia", func(args ...any) {
		payload := mapArg(args)
		if payload == nil {
			s.emitError(sock, "INVALID_INPUT", "media payload is required")
			return
		}
		s.handleUpdateStreamMedia(sock, payload)
	})

	sock.On("streamMessage", func(args ...any) {
		payload := mapArg(args)
		if payload == nil {
			s.emitError(sock, "INVALID_INPUT", "message payload is required")
			return
		}
		s.handleStreamMessage(sock, payload)
	})

	sock.On("streamSignal", func(args ...any) {
		payload := mapArg(args)
		if payload == nil {
			s.emitError(sock, "INVALID_INPUT", "signal payload is required")
			return
		}
		s.handleStreamSignal(sock, payload)
	})

	sock.On("pong", func(args ...any) {
		// optional: log latency when needed
		if len(args) > 0 {
			s.logger.Debug("pong received", slog.Any("value", args[0]))
		}
	})

	sock.On("disconnect", func(args ...any) {
		reason := "client"
		if len(args) > 0 {
			if r, ok := args[0].(string); ok {
				reason = r
			}
		}
		s.handleDisconnect(sock, reason)
	})
}

func (s *Server) handleGetActiveStreams(sock *socket.Socket) {
	streams := s.streamCache.GetAllStreams()
	payload := make([]map[string]any, 0, len(streams))
	for _, stream := range streams {
		if !stream.IsLive {
			continue
		}
		payload = append(payload, serializeStream(stream))
	}

	if err := sock.Emit("activeStreams", payload); err != nil {
		s.logger.Warn("failed to emit activeStreams", slog.String("error", err.Error()))
	}
}

func (s *Server) handleStartStream(sock *socket.Socket, payload map[string]any) {
	userData := s.getUserFromSocket(sock)
	if userData == nil {
		s.emitError(sock, "UNAUTHORIZED", "user context missing")
		return
	}

	streamID := strings.TrimSpace(stringValue(payload, "streamId"))
	title := strings.TrimSpace(stringValue(payload, "title"))
	description := strings.TrimSpace(stringValue(payload, "description"))
	chatEnabled := boolPointer(payload, "chatEnabled")
	isPublic := boolValue(payload, "isPublic", true)

	if streamID == "" || title == "" {
		s.emitError(sock, "INVALID_INPUT", "streamId and title are required")
		return
	}

	if existing, ok := s.streamCache.GetStream(streamID); ok && existing != nil && existing.IsLive {
		s.emitError(sock, "STREAM_EXISTS", "stream already exists")
		return
	}

	if err := s.validateStreamStart(userData.ID.String()); err != nil {
		s.emitError(sock, err.code, err.message)
		return
	}

	if total := len(s.streamCache.GetAllStreams()); total >= s.limits.MaxTotalConcurrentStreams {
		s.emitError(sock, "SERVER_BUSY", "too many active streams, try again later")
		return
	}

	sock.Join(streamRoom(streamID))

	opts := streamcache.StreamOptions{
		Title:       title,
		Description: description,
		HostName:    userData.FullName,
		IsPublic:    isPublic,
		ChatEnabled: chatEnabled,
	}

	stream := s.streamCache.StartStream(streamID, userData.ID.String(), opts)
	s.incrementStreamActivity(userData.ID.String())

	response := map[string]any{
		"streamId":  stream.ID,
		"stream":    serializeStream(*stream),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err := sock.Emit("streamStarted", response); err != nil {
		s.logger.Warn("failed to emit streamStarted", slog.String("error", err.Error()))
	}

	if stream.IsPublic {
		if err := sock.Broadcast().Emit("newStreamAvailable", map[string]any{
			"streamId":    stream.ID,
			"title":       stream.Title,
			"hostName":    stream.HostName,
			"viewerCount": stream.ViewerCount,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			s.logger.Warn("failed to broadcast new stream", slog.String("error", err.Error()))
		}
	}
}

func (s *Server) handleJoinStream(sock *socket.Socket, streamID string) {
	userData := s.getUserFromSocket(sock)
	if userData == nil {
		s.emitError(sock, "UNAUTHORIZED", "user context missing")
		return
	}

	stream, ok := s.streamCache.GetStream(streamID)
	if !ok || stream == nil {
		s.emitError(sock, "STREAM_NOT_FOUND", "stream not found")
		return
	}

	if !stream.IsLive {
		s.emitError(sock, "STREAM_NOT_LIVE", "stream is not live")
		return
	}

	if stream.ViewerCount >= s.limits.MaxViewersPerStream {
		s.emitError(sock, "STREAM_FULL", "stream is at maximum capacity")
		return
	}

	updated, err := s.streamCache.JoinStream(streamID, userData.ID.String())
	if err != nil {
		s.emitError(sock, "JOIN_FAILED", err.Error())
		return
	}

	sock.Join(streamRoom(streamID))

	payload := map[string]any{
		"streamId":  streamID,
		"stream":    serializeStream(*updated),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err := sock.Emit("streamJoined", payload); err != nil {
		s.logger.Warn("failed to emit streamJoined", slog.String("error", err.Error()))
	}

	if err := sock.To(streamRoom(streamID)).Emit("viewerJoined", map[string]any{
		"streamId":    streamID,
		"viewerId":    userData.ID.String(),
		"viewerName":  userData.FullName,
		"viewerCount": updated.ViewerCount,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		s.logger.Warn("failed to broadcast viewerJoined", slog.String("error", err.Error()))
	}
}

func (s *Server) handleLeaveStream(sock *socket.Socket, streamID, reason string) {
	userData := s.getUserFromSocket(sock)
	if userData == nil {
		return
	}

	sock.Leave(streamRoom(streamID))

	stream, err := s.streamCache.LeaveStream(streamID, userData.ID.String())
	if err != nil {
		if !strings.Contains(err.Error(), streamcache.ErrStreamNotFound.Error()) {
			s.logger.Warn("leaveStream error", slog.String("error", err.Error()))
		}
		return
	}

	if stream != nil && !stream.IsLive {
		s.decrementStreamActivity(userData.ID.String())
		s.broadcastStreamEnded(streamID, "host-ended")
		return
	}

	if stream != nil {
		if err := sock.To(streamRoom(streamID)).Emit("viewerLeft", map[string]any{
			"streamId":    streamID,
			"viewerId":    userData.ID.String(),
			"viewerName":  userData.FullName,
			"viewerCount": stream.ViewerCount,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"reason":      reason,
		}); err != nil {
			s.logger.Warn("failed to broadcast viewerLeft", slog.String("error", err.Error()))
		}
	}
}

func (s *Server) handleEndStream(sock *socket.Socket, streamID string) {
	userData := s.getUserFromSocket(sock)
	if userData == nil {
		s.emitError(sock, "UNAUTHORIZED", "user context missing")
		return
	}

	stream, ok := s.streamCache.GetStream(streamID)
	if !ok || stream == nil {
		s.emitError(sock, "STREAM_NOT_FOUND", "stream not found")
		return
	}

	if stream.HostID != userData.ID.String() {
		s.emitError(sock, "UNAUTHORIZED", "only the host can end the stream")
		return
	}

	if _, err := s.streamCache.EndStream(streamID); err != nil {
		s.emitError(sock, "END_FAILED", err.Error())
		return
	}

	s.decrementStreamActivity(userData.ID.String())
	s.broadcastStreamEnded(streamID, "host-ended")
}

func (s *Server) handleUpdateStreamMedia(sock *socket.Socket, payload map[string]any) {
	userData := s.getUserFromSocket(sock)
	if userData == nil {
		s.emitError(sock, "UNAUTHORIZED", "user context missing")
		return
	}

	streamID := strings.TrimSpace(stringValue(payload, "streamId"))
	if streamID == "" {
		s.emitError(sock, "INVALID_INPUT", "stream ID is required")
		return
	}

	stream, ok := s.streamCache.GetStream(streamID)
	if !ok || stream == nil {
		s.emitError(sock, "STREAM_NOT_FOUND", "stream not found")
		return
	}

	if stream.HostID != userData.ID.String() {
		s.emitError(sock, "UNAUTHORIZED", "only the host can update media state")
		return
	}

	updated, err := s.streamCache.UpdateStreamMedia(streamID, streamcache.MediaState{
		HasVideo:       boolPointer(payload, "hasVideo"),
		HasAudio:       boolPointer(payload, "hasAudio"),
		HasScreenShare: boolPointer(payload, "hasScreenShare"),
	})
	if err != nil {
		s.emitError(sock, "UPDATE_FAILED", err.Error())
		return
	}

	if err := sock.To(streamRoom(streamID)).Emit("streamMediaUpdated", map[string]any{
		"streamId":       streamID,
		"hasVideo":       updated.HasVideo,
		"hasAudio":       updated.HasAudio,
		"hasScreenShare": updated.HasScreenShare,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		s.logger.Warn("failed to broadcast media update", slog.String("error", err.Error()))
	}
}

func (s *Server) handleStreamMessage(sock *socket.Socket, payload map[string]any) {
	userData := s.getUserFromSocket(sock)
	if userData == nil {
		return
	}

	streamID := strings.TrimSpace(stringValue(payload, "streamId"))
	message := strings.TrimSpace(stringValue(payload, "message"))
	if streamID == "" || message == "" {
		s.emitError(sock, "INVALID_INPUT", "streamId and message are required")
		return
	}

	stream, ok := s.streamCache.GetStream(streamID)
	if !ok || stream == nil {
		s.emitError(sock, "STREAM_NOT_FOUND", "stream not found")
		return
	}

	chatMessage := map[string]any{
		"id":        fmt.Sprintf("%d", time.Now().UnixNano()),
		"streamId":  streamID,
		"userId":    userData.ID.String(),
		"userName":  userData.FullName,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"isHost":    stream.HostID == userData.ID.String(),
	}

	// Broadcast to everyone in the stream room including the sender
	// Using io.To() instead of sock.To() to ensure the sender also receives the message
	if err := s.io.To(streamRoom(streamID)).Emit("streamMessageReceived", chatMessage); err != nil {
		s.logger.Warn("failed to broadcast chat message", slog.String("error", err.Error()))
	}
}

func (s *Server) handleStreamSignal(sock *socket.Socket, payload map[string]any) {
	userData := s.getUserFromSocket(sock)
	if userData == nil {
		return
	}

	streamID := strings.TrimSpace(stringValue(payload, "streamId"))
	if streamID == "" {
		s.emitError(sock, "INVALID_INPUT", "stream ID is required")
		return
	}

	signal, ok := payload["signal"]
	if !ok {
		s.emitError(sock, "INVALID_INPUT", "signal payload is required")
		return
	}

	targetUserID := strings.TrimSpace(stringValue(payload, "targetUserId"))
	signalPayload := map[string]any{
		"streamId": streamID,
		"signal":   signal,
		"from":     userData.ID.String(),
	}

	if targetUserID != "" {
		if err := sock.To(userRoom(targetUserID)).Emit("streamSignal", signalPayload); err != nil {
			s.logger.Warn("failed to send direct stream signal", slog.String("error", err.Error()))
		}
		return
	}

	if err := sock.To(streamRoom(streamID)).Emit("streamSignal", signalPayload); err != nil {
		s.logger.Warn("failed to broadcast stream signal", slog.String("error", err.Error()))
	}
}

func (s *Server) handleDisconnect(sock *socket.Socket, reason string) {
	userData := s.getUserFromSocket(sock)

	s.connMutex.Lock()
	delete(s.connections, s.socketID(sock))
	s.connMutex.Unlock()

	if userData == nil {
		return
	}

	s.logger.Info("WebSocket disconnected",
		slog.String("user", userData.FullName),
		slog.String("userId", userData.ID.String()),
		slog.String("reason", reason),
	)

	streams := s.streamCache.GetAllStreams()
	for _, stream := range streams {
		switch {
		case stream.HostID == userData.ID.String():
			s.decrementStreamActivity(userData.ID.String())
			if _, err := s.streamCache.EndStream(stream.ID); err == nil {
				s.broadcastStreamEnded(stream.ID, "host-disconnected")
			}
		default:
			s.handleLeaveStream(sock, stream.ID, "disconnect")
		}
	}
}

func (s *Server) broadcastStreamEnded(streamID, reason string) {
	payload := map[string]any{
		"streamId":  streamID,
		"reason":    reason,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err := s.io.Local().To(streamRoom(streamID)).Emit("streamEnded", payload); err != nil {
		s.logger.Warn("failed to broadcast streamEnded", slog.String("error", err.Error()))
	}

	if err := s.io.Local().Emit("streamEnded", payload); err != nil {
		s.logger.Debug("failed to emit global streamEnded", slog.String("error", err.Error()))
	}
}

func (s *Server) startHeartbeat() {
	s.heartbeatStop = make(chan struct{})
	s.heartbeatWG.Add(1)

	go func() {
		defer s.heartbeatWG.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.sendHeartbeat()
			case <-s.heartbeatStop:
				return
			}
		}
	}()
}

func (s *Server) sendHeartbeat() {
	timestamp := time.Now().Unix()

	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	for id, sock := range s.connections {
		if err := sock.Emit("ping", timestamp); err != nil {
			s.logger.Debug("heartbeat emit failed", slog.String("connId", id), slog.String("error", err.Error()))
		}
	}
}

func (s *Server) getUserFromSocket(sock *socket.Socket) *user.User {
	if sock == nil {
		return nil
	}
	if data, ok := sock.Data().(*user.User); ok {
		return data
	}
	return nil
}

func (s *Server) emitError(sock *socket.Socket, code, message string) {
	if sock == nil {
		return
	}
	if err := sock.Emit("error", map[string]any{
		"code":    code,
		"message": message,
	}); err != nil {
		s.logger.Debug("failed to emit error", slog.String("error", err.Error()))
	}
}

type streamStartError struct {
	code    string
	message string
}

func (s *Server) validateStreamStart(userID string) *streamStartError {
	now := time.Now()

	s.activityMu.Lock()
	defer s.activityMu.Unlock()

	activity := s.userActivity[userID]
	if activity == nil {
		activity = &userStreamActivity{}
		s.userActivity[userID] = activity
	}

	if !activity.lastStreamStart.IsZero() && now.Sub(activity.lastStreamStart) < s.limits.StreamStartCooldown {
		remaining := s.limits.StreamStartCooldown - now.Sub(activity.lastStreamStart)
		return &streamStartError{code: "COOLDOWN", message: fmt.Sprintf("please wait %d seconds before starting another stream", int(remaining.Seconds()))}
	}

	hostStreams := s.countStreamsByHost(userID)
	if hostStreams >= s.limits.MaxConcurrentStreamsPerUser {
		return &streamStartError{code: "STREAM_LIMIT", message: "maximum concurrent streams reached"}
	}

	activity.lastStreamStart = now
	activity.activeStreams = hostStreams + 1

	return nil
}

func (s *Server) incrementStreamActivity(userID string) {
	s.activityMu.Lock()
	defer s.activityMu.Unlock()

	activity := s.userActivity[userID]
	if activity == nil {
		activity = &userStreamActivity{}
		s.userActivity[userID] = activity
	}
	activity.activeStreams++
	activity.lastStreamStart = time.Now()
}

func (s *Server) decrementStreamActivity(userID string) {
	s.activityMu.Lock()
	defer s.activityMu.Unlock()

	if activity := s.userActivity[userID]; activity != nil {
		if activity.activeStreams > 0 {
			activity.activeStreams--
		}
	}
}

func (s *Server) countStreamsByHost(userID string) int {
	total := 0
	for _, stream := range s.streamCache.GetAllStreams() {
		if stream.HostID == userID && stream.IsLive {
			total++
		}
	}
	return total
}

func (s *Server) extractToken(sock *socket.Socket) string {
	if sock == nil {
		return ""
	}

	if conn := sock.Conn(); conn != nil {
		if ctx := conn.Request(); ctx != nil {
			if req := ctx.Request(); req != nil {
				if token := req.URL.Query().Get("token"); token != "" {
					return token
				}
			}
			if query := ctx.Query(); query != nil {
				if token, ok := query.Get("token"); ok && token != "" {
					return token
				}
			}
		}
	}

	if hs := sock.Handshake(); hs != nil {
		if hs.Query != nil {
			if token, ok := hs.Query.Get("token"); ok && token != "" {
				return token
			}
		}
		if authMap, ok := hs.Auth.(map[string]any); ok {
			if token, ok := authMap["token"].(string); ok {
				return token
			}
		}
	}

	return ""
}

func (s *Server) socketID(sock *socket.Socket) string {
	if sock == nil {
		return ""
	}
	return string(sock.Id())
}

func serializeStream(stream streamcache.Stream) map[string]any {
	payload := map[string]any{
		"id":             stream.ID,
		"hostId":         stream.HostID,
		"hostName":       stream.HostName,
		"title":          stream.Title,
		"description":    stream.Description,
		"viewerCount":    stream.ViewerCount,
		"isLive":         stream.IsLive,
		"isPublic":       stream.IsPublic,
		"startTime":      stream.StartTime,
		"hasVideo":       stream.HasVideo,
		"hasAudio":       stream.HasAudio,
		"hasScreenShare": stream.HasScreenShare,
		"chatEnabled":    stream.ChatEnabled,
	}
	if stream.EndTime != nil {
		payload["endTime"] = stream.EndTime
	}
	return payload
}

func stringValue(payload map[string]any, key string) string {
	if val, ok := payload[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case fmt.Stringer:
			return v.String()
		case []byte:
			return string(v)
		}
	}
	return ""
}

func boolValue(payload map[string]any, key string, fallback bool) bool {
	if val, ok := payload[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			lower := strings.ToLower(strings.TrimSpace(v))
			if lower == "true" || lower == "1" {
				return true
			}
			if lower == "false" || lower == "0" {
				return false
			}
		}
	}
	return fallback
}

func boolPointer(payload map[string]any, key string) *bool {
	if val, ok := payload[key]; ok {
		switch v := val.(type) {
		case bool:
			return &v
		case string:
			lower := strings.ToLower(strings.TrimSpace(v))
			if lower == "true" || lower == "1" {
				b := true
				return &b
			}
			if lower == "false" || lower == "0" {
				b := false
				return &b
			}
		}
	}
	return nil
}

func stringArg(args []any) string {
	if len(args) == 0 {
		return ""
	}
	switch v := args[0].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case []byte:
		return string(v)
	}
	return ""
}

func mapArg(args []any) map[string]any {
	if len(args) == 0 {
		return nil
	}
	if payload, ok := args[0].(map[string]any); ok {
		return payload
	}
	return nil
}

func streamRoom(streamID string) socket.Room {
	return socket.Room("stream_" + streamID)
}

func userRoom(userID string) socket.Room {
	return socket.Room("user_" + userID)
}
