package streamcache

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrStreamNotFound indicates that the requested stream is missing from the cache.
	ErrStreamNotFound = errors.New("stream not found")
)

// Stream captures the public information about a live stream session.
type Stream struct {
	ID             string     `json:"id"`
	HostID         string     `json:"hostId"`
	HostName       string     `json:"hostName"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	ViewerCount    int        `json:"viewerCount"`
	IsLive         bool       `json:"isLive"`
	IsPublic       bool       `json:"isPublic"`
	StartTime      time.Time  `json:"startTime"`
	EndTime        *time.Time `json:"endTime,omitempty"`
	HasVideo       bool       `json:"hasVideo"`
	HasAudio       bool       `json:"hasAudio"`
	HasScreenShare bool       `json:"hasScreenShare"`
	ChatEnabled    bool       `json:"chatEnabled"`
}

// StreamOptions configures a new stream when it is started.
type StreamOptions struct {
	Title       string
	Description string
	HostName    string
	IsPublic    bool
	ChatEnabled *bool
}

// MediaState updates the media flags for a running stream.
type MediaState struct {
	HasVideo       *bool
	HasAudio       *bool
	HasScreenShare *bool
}

// Cache is an in-memory registry of active streams.
type Cache struct {
	mu      sync.RWMutex
	streams map[string]*Stream
	viewers map[string]map[string]struct{}
	hosts   map[string]string
}

var globalCache = New()

// Global returns the shared cache instance used across the application.
func Global() *Cache {
	return globalCache
}

// New constructs an empty stream cache.
func New() *Cache {
	return &Cache{
		streams: make(map[string]*Stream),
		viewers: make(map[string]map[string]struct{}),
		hosts:   make(map[string]string),
	}
}

// StartStream registers a new live stream hosted by hostID.
func (c *Cache) StartStream(streamID, hostID string, opts StreamOptions) *Stream {
	c.mu.Lock()
	defer c.mu.Unlock()

	enabledChat := true
	if opts.ChatEnabled != nil {
		enabledChat = *opts.ChatEnabled
	}

	stream := &Stream{
		ID:             streamID,
		HostID:         hostID,
		HostName:       opts.HostName,
		Title:          defaultString(opts.Title, "Live Stream"),
		Description:    opts.Description,
		ViewerCount:    0,
		IsLive:         true,
		IsPublic:       opts.IsPublic,
		StartTime:      time.Now().UTC(),
		HasVideo:       false,
		HasAudio:       false,
		HasScreenShare: false,
		ChatEnabled:    enabledChat,
	}

	c.streams[streamID] = stream
	c.viewers[streamID] = make(map[string]struct{})
	c.hosts[streamID] = hostID

	copy := *stream
	return &copy
}

// JoinStream adds a viewer to the stream's audience.
func (c *Cache) JoinStream(streamID, viewerID string) (*Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream, ok := c.streams[streamID]
	if !ok || !stream.IsLive {
		return nil, ErrStreamNotFound
	}

	viewers := c.ensureViewerSet(streamID)
	if _, exists := viewers[viewerID]; !exists {
		viewers[viewerID] = struct{}{}
		stream.ViewerCount = len(viewers)
	}

	copy := *stream
	return &copy, nil
}

// LeaveStream removes a viewer or ends the stream if the host leaves.
func (c *Cache) LeaveStream(streamID, userID string) (*Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream, ok := c.streams[streamID]
	if !ok {
		return nil, ErrStreamNotFound
	}

	hostID := c.hosts[streamID]
	if userID == hostID {
		return c.endStreamLocked(streamID, stream)
	}

	if viewers, exists := c.viewers[streamID]; exists {
		if _, watching := viewers[userID]; watching {
			delete(viewers, userID)
			stream.ViewerCount = len(viewers)
		}
	}

	copy := *stream
	return &copy, nil
}

// EndStream terminates a stream immediately.
func (c *Cache) EndStream(streamID string) (*Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream, ok := c.streams[streamID]
	if !ok {
		return nil, ErrStreamNotFound
	}

	return c.endStreamLocked(streamID, stream)
}

// UpdateStreamMedia updates the media state flags for the given stream.
func (c *Cache) UpdateStreamMedia(streamID string, media MediaState) (*Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream, ok := c.streams[streamID]
	if !ok || !stream.IsLive {
		return nil, ErrStreamNotFound
	}

	if media.HasVideo != nil {
		stream.HasVideo = *media.HasVideo
	}
	if media.HasAudio != nil {
		stream.HasAudio = *media.HasAudio
	}
	if media.HasScreenShare != nil {
		stream.HasScreenShare = *media.HasScreenShare
	}

	copy := *stream
	return &copy, nil
}

// GetStream retrieves a copy of the stream if it exists.
func (c *Cache) GetStream(streamID string) (*Stream, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stream, ok := c.streams[streamID]
	if !ok {
		return nil, false
	}

	copy := *stream
	return &copy, true
}

// GetAllStreams returns snapshots of all live streams currently registered.
func (c *Cache) GetAllStreams() []Stream {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Stream, 0, len(c.streams))
	for _, stream := range c.streams {
		if !stream.IsLive {
			continue
		}
		copy := *stream
		result = append(result, copy)
	}
	return result
}

// Reset clears the cache. Primarily useful for tests.
func (c *Cache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.streams = make(map[string]*Stream)
	c.viewers = make(map[string]map[string]struct{})
	c.hosts = make(map[string]string)
}

func (c *Cache) ensureViewerSet(streamID string) map[string]struct{} {
	if viewers, ok := c.viewers[streamID]; ok {
		return viewers
	}
	viewers := make(map[string]struct{})
	c.viewers[streamID] = viewers
	return viewers
}

func (c *Cache) endStreamLocked(streamID string, stream *Stream) (*Stream, error) {
	now := time.Now().UTC()
	stream.IsLive = false
	stream.EndTime = &now

	copy := *stream

	delete(c.streams, streamID)
	delete(c.viewers, streamID)
	delete(c.hosts, streamID)

	return &copy, nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
