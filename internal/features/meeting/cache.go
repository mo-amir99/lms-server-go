package meeting

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Meeting represents an active meeting
type Meeting struct {
	RoomID             string                  `json:"roomId"`
	SubscriptionID     string                  `json:"subscriptionId"`
	Title              string                  `json:"title"`
	Description        string                  `json:"description"`
	HostID             string                  `json:"hostId"`
	AccessType         string                  `json:"accessType"` // "public" or "group"
	GroupAccess        []string                `json:"groupAccess"`
	Participants       map[string]*Participant `json:"participants"`
	StartedAt          time.Time               `json:"startedAt"`
	Status             string                  `json:"status"` // "active" or "ended"
	StudentPermissions StudentPermissions      `json:"studentPermissions"`
}

// Participant represents a meeting participant
type Participant struct {
	ID          string `json:"id"`
	IDString    string `json:"_id"` // For compatibility with Node.js
	Name        string `json:"name"`
	Email       string `json:"email"`
	Mic         bool   `json:"mic"`
	Camera      bool   `json:"camera"`
	ScreenShare bool   `json:"screenShare"`
}

// StudentPermissions represents what students can do in the meeting
type StudentPermissions struct {
	CanUseMic      bool `json:"canUseMic"`
	CanUseCamera   bool `json:"canUseCamera"`
	CanScreenShare bool `json:"canScreenShare"`
}

// Cache is an in-memory meeting cache
type Cache struct {
	mu                   sync.RWMutex
	meetings             map[string]*Meeting        // roomId -> meeting
	subscriptionMeetings map[string]map[string]bool // subscriptionId -> set of roomIds
	userMeetings         map[string]map[string]bool // userId -> set of roomIds
}

// NewCache creates a new meeting cache
func NewCache() *Cache {
	return &Cache{
		meetings:             make(map[string]*Meeting),
		subscriptionMeetings: make(map[string]map[string]bool),
		userMeetings:         make(map[string]map[string]bool),
	}
}

// CreateMeeting creates a new meeting
func (c *Cache) CreateMeeting(input CreateMeetingInput) (*Meeting, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if room already exists
	if _, exists := c.meetings[input.RoomID]; exists {
		return nil, errors.New("Meeting with this room ID already exists")
	}

	// Check if subscription already has an active meeting
	if existingRooms, exists := c.subscriptionMeetings[input.SubscriptionID]; exists && len(existingRooms) > 0 {
		return nil, errors.New("Subscription already has an active meeting")
	}

	// Create meeting
	meeting := &Meeting{
		RoomID:         input.RoomID,
		SubscriptionID: input.SubscriptionID,
		Title:          input.Title,
		Description:    input.Description,
		HostID:         input.HostID,
		AccessType:     input.AccessType,
		GroupAccess:    input.GroupAccess,
		Participants:   make(map[string]*Participant),
		StartedAt:      time.Now(),
		Status:         "active",
		StudentPermissions: StudentPermissions{
			CanUseMic:      false,
			CanUseCamera:   false,
			CanScreenShare: false,
		},
	}

	// Store meeting
	c.meetings[input.RoomID] = meeting

	// Update subscription index
	if c.subscriptionMeetings[input.SubscriptionID] == nil {
		c.subscriptionMeetings[input.SubscriptionID] = make(map[string]bool)
	}
	c.subscriptionMeetings[input.SubscriptionID][input.RoomID] = true

	// Update host's user index
	if c.userMeetings[input.HostID] == nil {
		c.userMeetings[input.HostID] = make(map[string]bool)
	}
	c.userMeetings[input.HostID][input.RoomID] = true

	return meeting, nil
}

// CreateMeetingInput represents the input for creating a meeting
type CreateMeetingInput struct {
	RoomID         string
	SubscriptionID string
	Title          string
	Description    string
	HostID         string
	AccessType     string
	GroupAccess    []string
}

// AddParticipant adds a participant to a meeting
func (c *Cache) AddParticipant(roomID, userID string, details *Participant) {
	c.mu.Lock()
	defer c.mu.Unlock()

	meeting, exists := c.meetings[roomID]
	if !exists {
		return
	}

	// Ensure IDs are set
	if details != nil {
		if details.ID == "" {
			details.ID = userID
		}
		if details.IDString == "" {
			details.IDString = userID
		}
		meeting.Participants[userID] = details
	}
}

// GetMeeting retrieves a meeting by room ID
func (c *Cache) GetMeeting(roomID string) *Meeting {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.meetings[roomID]
}

// GetSubscriptionMeetings retrieves all meetings for a subscription
func (c *Cache) GetSubscriptionMeetings(subscriptionID string) []*Meeting {
	c.mu.RLock()
	defer c.mu.RUnlock()

	roomIDs, exists := c.subscriptionMeetings[subscriptionID]
	if !exists {
		return []*Meeting{}
	}

	meetings := make([]*Meeting, 0, len(roomIDs))
	for roomID := range roomIDs {
		if meeting, ok := c.meetings[roomID]; ok {
			meetings = append(meetings, meeting)
		}
	}

	return meetings
}

// JoinMeeting adds a user to a meeting
func (c *Cache) JoinMeeting(roomID, userID string, details *Participant) (*Meeting, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	meeting, exists := c.meetings[roomID]
	if !exists {
		return nil, errors.New("Meeting not found")
	}

	if meeting.Status != "active" {
		return nil, errors.New("Meeting is not active")
	}

	// Add participant
	if details == nil {
		details = &Participant{
			ID:          userID,
			IDString:    userID,
			Name:        "Unknown",
			Email:       "",
			Mic:         true,
			Camera:      true,
			ScreenShare: false,
		}
	} else {
		if details.ID == "" {
			details.ID = userID
		}
		if details.IDString == "" {
			details.IDString = userID
		}
	}

	meeting.Participants[userID] = details

	// Update user meeting index
	if c.userMeetings[userID] == nil {
		c.userMeetings[userID] = make(map[string]bool)
	}
	c.userMeetings[userID][roomID] = true

	return meeting, nil
}

// LeaveMeeting removes a user from a meeting
func (c *Cache) LeaveMeeting(roomID, userID string) (found, autoClosedMeeting bool, meeting *Meeting) {
	c.mu.Lock()
	defer c.mu.Unlock()

	meeting, exists := c.meetings[roomID]
	if !exists {
		return false, false, nil
	}

	// Remove participant
	delete(meeting.Participants, userID)

	// Update user meeting index
	if userRooms, ok := c.userMeetings[userID]; ok {
		delete(userRooms, roomID)
		if len(userRooms) == 0 {
			delete(c.userMeetings, userID)
		}
	}

	// Auto-close if empty
	if len(meeting.Participants) == 0 {
		c.endMeetingUnsafe(roomID)
		return true, true, meeting
	}

	return true, false, meeting
}

// EndMeeting ends a meeting
func (c *Cache) EndMeeting(roomID string) (found bool, meeting *Meeting) {
	c.mu.Lock()
	defer c.mu.Unlock()

	meeting, exists := c.meetings[roomID]
	if !exists {
		return false, nil
	}

	c.endMeetingUnsafe(roomID)
	return true, meeting
}

// endMeetingUnsafe ends a meeting without locking (internal use)
func (c *Cache) endMeetingUnsafe(roomID string) {
	meeting, exists := c.meetings[roomID]
	if !exists {
		return
	}

	// Clean up participants from user index
	for userID := range meeting.Participants {
		if userRooms, ok := c.userMeetings[userID]; ok {
			delete(userRooms, roomID)
			if len(userRooms) == 0 {
				delete(c.userMeetings, userID)
			}
		}
	}

	// Remove from subscription index
	if subscriptionRooms, ok := c.subscriptionMeetings[meeting.SubscriptionID]; ok {
		delete(subscriptionRooms, roomID)
		if len(subscriptionRooms) == 0 {
			delete(c.subscriptionMeetings, meeting.SubscriptionID)
		}
	}

	// Remove meeting
	delete(c.meetings, roomID)
}

// UpdatePermissions updates student permissions for a meeting
func (c *Cache) UpdatePermissions(roomID string, permissions StudentPermissions) (*Meeting, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	meeting, exists := c.meetings[roomID]
	if !exists {
		return nil, errors.New("Meeting not found")
	}

	meeting.StudentPermissions = permissions
	return meeting, nil
}

// UpdateParticipantMedia updates a participant's media state
func (c *Cache) UpdateParticipantMedia(roomID, userID string, mic, camera, screenShare *bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	meeting, exists := c.meetings[roomID]
	if !exists {
		return
	}

	participant, exists := meeting.Participants[userID]
	if !exists {
		return
	}

	if mic != nil {
		participant.Mic = *mic
	}
	if camera != nil {
		participant.Camera = *camera
	}
	if screenShare != nil {
		participant.ScreenShare = *screenShare
	}
}

// GetStats returns cache statistics
func (c *Cache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"totalActiveMeetings":       len(c.meetings),
		"subscriptionsWithMeetings": len(c.subscriptionMeetings),
		"usersInMeetings":           len(c.userMeetings),
	}
}

// GenerateRoomID generates a unique room ID
func GenerateRoomID() string {
	return uuid.New().String()
}
