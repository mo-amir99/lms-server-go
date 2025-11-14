package meeting

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/groupaccess"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
	cache  *Cache
}

func NewHandler(db *gorm.DB, logger *slog.Logger, cache *Cache) *Handler {
	return &Handler{
		db:     db,
		logger: logger,
		cache:  cache,
	}
}

// CreateMeeting creates and starts a new meeting
// POST /subscriptions/:subscriptionId/meetings
func (h *Handler) CreateMeeting(c *gin.Context) {
	subscriptionID := c.Param("subscriptionId")

	// Parse request body
	var req struct {
		Title       string   `json:"title" binding:"required"`
		Description string   `json:"description"`
		AccessType  string   `json:"accessType"` // "public" or "group"
		GroupAccess []string `json:"groupAccess"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Get user from context
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Validate subscription exists
	var sub subscription.Subscription
	if err := h.db.Where("id = ?", subscriptionID).First(&sub).Error; err != nil {
		response.Error(c, http.StatusNotFound, "Subscription not found", nil)
		return
	}

	// Check if user belongs to this subscription
	if currentUser.SubscriptionID == nil || currentUser.SubscriptionID.String() != subscriptionID {
		response.Error(c, http.StatusForbidden, "You can only create meetings for your own subscription", nil)
		return
	}

	// Validate group access if needed
	if req.AccessType == "group" && len(req.GroupAccess) > 0 {
		var validGroups []groupaccess.GroupAccess
		if err := h.db.Where("id IN ? AND subscription_id = ?", req.GroupAccess, subscriptionID).Find(&validGroups).Error; err != nil {
			h.logger.Error("Failed to validate groups", "error", err)
		}
		if len(validGroups) != len(req.GroupAccess) {
			response.Error(c, http.StatusBadRequest, "One or more invalid group IDs provided", nil)
			return
		}
	}

	// Generate room ID using subscription identifier
	roomID := sub.IdentifierName
	if roomID == "" {
		roomID = GenerateRoomID()
	}

	// Create meeting in cache
	meeting, err := h.cache.CreateMeeting(CreateMeetingInput{
		RoomID:         roomID,
		SubscriptionID: subscriptionID,
		Title:          req.Title,
		Description:    req.Description,
		HostID:         currentUser.ID.String(),
		AccessType:     req.AccessType,
		GroupAccess:    req.GroupAccess,
	})

	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// Add host as first participant
	h.cache.AddParticipant(roomID, currentUser.ID.String(), &Participant{
		ID:          currentUser.ID.String(),
		IDString:    currentUser.ID.String(),
		Name:        currentUser.FullName,
		Email:       currentUser.Email,
		Mic:         true,
		Camera:      true,
		ScreenShare: true,
	})

	// Convert participants map to array for response
	participants := make([]*Participant, 0, len(meeting.Participants))
	for _, p := range meeting.Participants {
		participants = append(participants, p)
	}

	responseData := gin.H{
		"roomId":             meeting.RoomID,
		"subscriptionId":     meeting.SubscriptionID,
		"title":              meeting.Title,
		"description":        meeting.Description,
		"hostId":             meeting.HostID,
		"accessType":         meeting.AccessType,
		"groupAccess":        meeting.GroupAccess,
		"participants":       participants,
		"startedAt":          meeting.StartedAt,
		"status":             meeting.Status,
		"studentPermissions": meeting.StudentPermissions,
		"host": gin.H{
			"_id":   currentUser.ID,
			"id":    currentUser.ID,
			"name":  currentUser.FullName,
			"email": currentUser.Email,
		},
	}

	response.Created(c, responseData, "Meeting created and started successfully")
}

// GetActiveMeetings returns all active meetings for a subscription
// GET /subscriptions/:subscriptionId/meetings
func (h *Handler) GetActiveMeetings(c *gin.Context) {
	subscriptionID := c.Param("subscriptionId")

	meetings := h.cache.GetSubscriptionMeetings(subscriptionID)

	// Convert participants maps to arrays
	responseData := make([]gin.H, 0, len(meetings))
	for _, meeting := range meetings {
		participants := make([]*Participant, 0, len(meeting.Participants))
		for _, p := range meeting.Participants {
			participants = append(participants, p)
		}

		responseData = append(responseData, gin.H{
			"roomId":             meeting.RoomID,
			"subscriptionId":     meeting.SubscriptionID,
			"title":              meeting.Title,
			"description":        meeting.Description,
			"hostId":             meeting.HostID,
			"accessType":         meeting.AccessType,
			"groupAccess":        meeting.GroupAccess,
			"participants":       participants,
			"startedAt":          meeting.StartedAt,
			"status":             meeting.Status,
			"studentPermissions": meeting.StudentPermissions,
		})
	}

	response.Success(c, http.StatusOK, responseData, "", nil)
}

// GetMeetingByRoomID returns a meeting by its room ID
// GET /meetings/:roomId
func (h *Handler) GetMeetingByRoomID(c *gin.Context) {
	roomID := c.Param("roomId")

	meeting := h.cache.GetMeeting(roomID)
	if meeting == nil {
		response.Error(c, http.StatusNotFound, "Meeting not found", nil)
		return
	}

	// Convert participants map to array
	participants := make([]*Participant, 0, len(meeting.Participants))
	for _, p := range meeting.Participants {
		participants = append(participants, p)
	}

	responseData := gin.H{
		"roomId":             meeting.RoomID,
		"subscriptionId":     meeting.SubscriptionID,
		"title":              meeting.Title,
		"description":        meeting.Description,
		"hostId":             meeting.HostID,
		"accessType":         meeting.AccessType,
		"groupAccess":        meeting.GroupAccess,
		"participants":       participants,
		"startedAt":          meeting.StartedAt,
		"status":             meeting.Status,
		"studentPermissions": meeting.StudentPermissions,
	}

	response.Success(c, http.StatusOK, responseData, "", nil)
}

// JoinMeeting allows a user to join a meeting
// POST /meetings/:roomId/join
func (h *Handler) JoinMeeting(c *gin.Context) {
	roomID := c.Param("roomId")

	// Get user from context
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Join meeting
	meeting, err := h.cache.JoinMeeting(roomID, currentUser.ID.String(), &Participant{
		ID:          currentUser.ID.String(),
		IDString:    currentUser.ID.String(),
		Name:        currentUser.FullName,
		Email:       currentUser.Email,
		Mic:         true,
		Camera:      true,
		ScreenShare: false,
	})

	if err != nil {
		if err.Error() == "Meeting not found" {
			response.Error(c, http.StatusNotFound, "Meeting not found", nil)
		} else if err.Error() == "Meeting is not active" {
			response.Error(c, http.StatusBadRequest, "Meeting is not active", nil)
		} else {
			response.Error(c, http.StatusInternalServerError, err.Error(), nil)
		}
		return
	}

	// Convert participants map to array
	participants := make([]*Participant, 0, len(meeting.Participants))
	for _, p := range meeting.Participants {
		participants = append(participants, p)
	}

	responseData := gin.H{
		"roomId":             meeting.RoomID,
		"subscriptionId":     meeting.SubscriptionID,
		"title":              meeting.Title,
		"description":        meeting.Description,
		"hostId":             meeting.HostID,
		"accessType":         meeting.AccessType,
		"groupAccess":        meeting.GroupAccess,
		"participants":       participants,
		"startedAt":          meeting.StartedAt,
		"status":             meeting.Status,
		"studentPermissions": meeting.StudentPermissions,
	}

	response.Success(c, http.StatusOK, responseData, "Successfully joined the meeting", nil)
}

// LeaveMeeting allows a user to leave a meeting
// POST /meetings/:roomId/leave
func (h *Handler) LeaveMeeting(c *gin.Context) {
	roomID := c.Param("roomId")

	// Get user from context
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	found, autoClosedMeeting, _ := h.cache.LeaveMeeting(roomID, currentUser.ID.String())

	if !found {
		response.Error(c, http.StatusNotFound, "Meeting not found", nil)
		return
	}

	message := "Successfully left the meeting"
	responseData := gin.H{
		"meetingEnded": autoClosedMeeting,
	}

	if autoClosedMeeting {
		message = "Successfully left the meeting. Meeting was automatically ended as it became empty"
	}

	response.Success(c, http.StatusOK, responseData, message, nil)
}

// UpdateStudentPermissions updates what students can do in the meeting (host only)
// PATCH /meetings/:roomId/permissions
func (h *Handler) UpdateStudentPermissions(c *gin.Context) {
	roomID := c.Param("roomId")

	// Parse request body
	var req StudentPermissions
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Get user from context
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Get meeting to check host
	meeting := h.cache.GetMeeting(roomID)
	if meeting == nil {
		response.Error(c, http.StatusNotFound, "Meeting not found", nil)
		return
	}

	// Check if user is the host (or admin/superadmin)
	isHost := meeting.HostID == currentUser.ID.String()
	isAdmin := currentUser.UserType == types.UserTypeAdmin || currentUser.UserType == types.UserTypeSuperAdmin

	if !isHost && !isAdmin {
		response.Error(c, http.StatusForbidden, "Only the meeting host can update student permissions", nil)
		return
	}

	// Update permissions
	updatedMeeting, err := h.cache.UpdatePermissions(roomID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, updatedMeeting.StudentPermissions, "Student permissions updated successfully", nil)
}

// EndMeeting ends a meeting (host only)
// POST /meetings/:roomId/end
func (h *Handler) EndMeeting(c *gin.Context) {
	roomID := c.Param("roomId")

	// Get user from context
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Get meeting to check host
	meeting := h.cache.GetMeeting(roomID)
	if meeting == nil {
		response.Error(c, http.StatusNotFound, "Meeting not found", nil)
		return
	}

	// Check if user is the host (or admin/superadmin)
	isHost := meeting.HostID == currentUser.ID.String()
	isAdmin := currentUser.UserType == types.UserTypeAdmin || currentUser.UserType == types.UserTypeSuperAdmin

	if !isHost && !isAdmin {
		response.Error(c, http.StatusForbidden, "Only the meeting host can end the meeting", nil)
		return
	}

	// End meeting
	found, endedMeeting := h.cache.EndMeeting(roomID)
	if !found {
		response.Error(c, http.StatusNotFound, "Meeting not found", nil)
		return
	}

	// Convert participants map to array
	participants := make([]*Participant, 0, len(endedMeeting.Participants))
	for _, p := range endedMeeting.Participants {
		participants = append(participants, p)
	}

	responseData := gin.H{
		"roomId":             endedMeeting.RoomID,
		"subscriptionId":     endedMeeting.SubscriptionID,
		"title":              endedMeeting.Title,
		"description":        endedMeeting.Description,
		"hostId":             endedMeeting.HostID,
		"accessType":         endedMeeting.AccessType,
		"groupAccess":        endedMeeting.GroupAccess,
		"participants":       participants,
		"startedAt":          endedMeeting.StartedAt,
		"status":             "ended",
		"studentPermissions": endedMeeting.StudentPermissions,
	}

	response.Success(c, http.StatusOK, responseData, "Meeting ended successfully", nil)
}
