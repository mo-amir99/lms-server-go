package thread

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Reply represents a nested reply in a thread.
type Reply struct {
	ID        string    `json:"id"`
	UserName  string    `json:"userName"`
	UserType  string    `json:"userType"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// Thread represents a discussion thread in a forum.
type Thread struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ForumID   uuid.UUID       `gorm:"type:uuid;not null;index:idx_forum_approved_created,priority:1;index:idx_forum_created,priority:1" json:"forumId"`
	Title     string          `gorm:"size:100;not null" json:"title"`
	Content   string          `gorm:"size:2000;not null" json:"content"`
	UserName  string          `gorm:"size:30;not null;index:idx_username_created,priority:1" json:"userName"`
	UserType  string          `gorm:"size:20;not null" json:"userType"`
	Replies   json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"replies"`
	Approved  bool            `gorm:"default:true;not null;index:idx_forum_approved_created,priority:2" json:"isApproved"`
	CreatedAt time.Time       `gorm:"not null;index:idx_forum_approved_created,priority:3;index:idx_forum_created,priority:2;index:idx_username_created,priority:2" json:"createdAt"`
	UpdatedAt time.Time       `gorm:"not null" json:"updatedAt"`
}

// TableName overrides the default table name.
func (Thread) TableName() string {
	return "threads"
}

// GetRecentForForum retrieves up to 20 recent approved threads for a forum (excluding replies).
func GetRecentForForum(db *gorm.DB, forumID uuid.UUID, limit int) ([]Thread, error) {
	if limit == 0 {
		limit = 20
	}

	var threads []Thread
	err := db.Where("forum_id = ? AND approved = ?", forumID, true).
		Order("created_at DESC").
		Limit(limit).
		Select("id, forum_id, title, content, user_name, user_type, approved, created_at, updated_at").
		Find(&threads).Error

	return threads, err
}

// GetByForum retrieves all approved threads for a forum with pagination.
func GetByForum(db *gorm.DB, forumID uuid.UUID, limit, offset int) ([]Thread, int64, error) {
	var threads []Thread
	var total int64

	query := db.Where("forum_id = ? AND approved = ?", forumID, true)

	if err := query.Model(&Thread{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Select("id, forum_id, title, content, user_name, user_type, approved, created_at, updated_at").
		Find(&threads).Error; err != nil {
		return nil, 0, err
	}

	return threads, total, nil
}

// Get retrieves a single thread by ID with full replies.
func Get(db *gorm.DB, id uuid.UUID) (*Thread, error) {
	var thread Thread
	if err := db.First(&thread, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrThreadNotFound
		}
		return nil, err
	}
	return &thread, nil
}

// CreateInput defines the payload to create a thread.
type CreateInput struct {
	ForumID  uuid.UUID
	Title    string
	Content  string
	UserName string
	UserType types.UserType
	Approved *bool
}

// Create inserts a new thread.
func Create(db *gorm.DB, input CreateInput) (*Thread, error) {
	if input.Title == "" {
		return nil, ErrTitleRequired
	}
	if input.Content == "" {
		return nil, ErrContentRequired
	}
	if input.UserName == "" {
		return nil, ErrUserNameRequired
	}

	thread := Thread{
		ForumID:  input.ForumID,
		Title:    input.Title,
		Content:  input.Content,
		UserName: input.UserName,
		UserType: string(input.UserType), // Convert typed enum to string for storage
		Replies:  json.RawMessage("[]"),
		Approved: true,
	}

	if input.Approved != nil {
		thread.Approved = *input.Approved
	}

	if err := db.Create(&thread).Error; err != nil {
		return nil, err
	}

	return &thread, nil
}

// UpdateInput defines the payload to update a thread.
type UpdateInput struct {
	Title    *string
	Content  *string
	Approved *bool
}

// Update modifies an existing thread.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (*Thread, error) {
	var thread Thread
	if err := db.First(&thread, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrThreadNotFound
		}
		return nil, err
	}

	updates := map[string]interface{}{}

	if input.Title != nil {
		if *input.Title == "" {
			return nil, ErrTitleRequired
		}
		updates["title"] = *input.Title
	}

	if input.Content != nil {
		if *input.Content == "" {
			return nil, ErrContentRequired
		}
		updates["content"] = *input.Content
	}

	if input.Approved != nil {
		updates["approved"] = *input.Approved
	}

	if len(updates) > 0 {
		if err := db.Model(&thread).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return &thread, nil
}

// Delete removes a thread.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Thread{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrThreadNotFound
	}

	return nil
}

// AddReply adds a reply to a thread.
func AddReply(db *gorm.DB, threadID uuid.UUID, userName string, userType types.UserType, content string) (*Thread, error) {
	var thread Thread
	if err := db.First(&thread, "id = ?", threadID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrThreadNotFound
		}
		return nil, err
	}

	var replies []Reply
	if err := json.Unmarshal(thread.Replies, &replies); err != nil {
		// If unmarshal fails, start fresh
		replies = []Reply{}
	}

	newReply := Reply{
		ID:        uuid.New().String(),
		UserName:  userName,
		UserType:  string(userType), // Convert typed enum to string for JSON storage
		Content:   content,
		CreatedAt: time.Now(),
	}

	replies = append(replies, newReply)

	repliesJSON, err := json.Marshal(replies)
	if err != nil {
		return nil, err
	}

	if err := db.Model(&thread).Update("replies", repliesJSON).Error; err != nil {
		return nil, err
	}

	thread.Replies = repliesJSON
	return &thread, nil
}

// DeleteReply removes a reply from a thread.
func DeleteReply(db *gorm.DB, threadID uuid.UUID, replyID string) (*Thread, error) {
	var thread Thread
	if err := db.First(&thread, "id = ?", threadID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrThreadNotFound
		}
		return nil, err
	}

	var replies []Reply
	if err := json.Unmarshal(thread.Replies, &replies); err != nil {
		return nil, err
	}

	found := false
	newReplies := []Reply{}
	for _, reply := range replies {
		if reply.ID != replyID {
			newReplies = append(newReplies, reply)
		} else {
			found = true
		}
	}

	if !found {
		return nil, ErrReplyNotFound
	}

	repliesJSON, err := json.Marshal(newReplies)
	if err != nil {
		return nil, err
	}

	if err := db.Model(&thread).Update("replies", repliesJSON).Error; err != nil {
		return nil, err
	}

	thread.Replies = repliesJSON
	return &thread, nil
}
