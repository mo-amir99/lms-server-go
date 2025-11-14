package forum

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Forum represents a discussion forum under a subscription.
type Forum struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SubscriptionID   uuid.UUID `gorm:"type:uuid;not null;index:idx_subscription_order" json:"subscriptionId"`
	Title            string    `gorm:"size:100;not null" json:"title"`
	Description      *string   `gorm:"size:600" json:"description,omitempty"`
	AssistantsOnly   bool      `gorm:"default:false;not null" json:"assistantsOnly"`
	RequiresApproval bool      `gorm:"default:false;not null" json:"requiresApproval"`
	Active           bool      `gorm:"default:true;not null;index:idx_subscription_active" json:"isActive"`
	Order            int       `gorm:"default:0;not null;index:idx_subscription_order" json:"order"`
	CreatedAt        time.Time `gorm:"not null" json:"createdAt"`
	UpdatedAt        time.Time `gorm:"not null" json:"updatedAt"`
}

// TableName overrides the default table name.
func (Forum) TableName() string {
	return "forums"
}

// List retrieves paginated forums for a subscription.
// If userType is STUDENT, only active forums are returned.
func List(db *gorm.DB, subscriptionID uuid.UUID, userType types.UserType, params pagination.Params) ([]Forum, int64, error) {
	query := db.Model(&Forum{}).Where("subscription_id = ?", subscriptionID)

	if userType == types.UserTypeStudent {
		query = query.Where("active = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var forums []Forum
	err := query.
		Order("\"order\" ASC, created_at DESC").
		Offset(params.Skip).
		Limit(params.Limit).
		Find(&forums).Error

	return forums, total, err
}

// GetBySubscription retrieves all forums for a subscription.
// If userType is STUDENT, only active forums are returned.
func GetBySubscription(db *gorm.DB, subscriptionID uuid.UUID, userType types.UserType) ([]Forum, error) {
	var forums []Forum
	query := db.Where("subscription_id = ?", subscriptionID)

	if userType == types.UserTypeStudent {
		query = query.Where("active = ?", true)
	}

	if err := query.Order("\"order\" ASC, created_at DESC").Find(&forums).Error; err != nil {
		return nil, err
	}

	return forums, nil
}

// Get retrieves a single forum by ID.
func Get(db *gorm.DB, id uuid.UUID) (*Forum, error) {
	var forum Forum
	if err := db.First(&forum, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrForumNotFound
		}
		return nil, err
	}
	return &forum, nil
}

// ForumWithThreads represents a forum with recent threads.
type ForumWithThreads struct {
	Forum
	Threads interface{} `json:"threads"`
}

// GetWithThreads retrieves a forum with up to 20 recent approved threads (excluding replies).
func GetWithThreads(db *gorm.DB, id uuid.UUID) (*ForumWithThreads, error) {
	forum, err := Get(db, id)
	if err != nil {
		return nil, err
	}

	// Get recent threads (need to import thread package, but this causes import cycle)
	// Using direct query instead
	type ThreadSummary struct {
		ID        uuid.UUID `json:"id"`
		ForumID   uuid.UUID `json:"forumId"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		UserName  string    `json:"userName"`
		UserType  string    `json:"userType"`
		Approved  bool      `json:"isApproved"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}

	var threads []ThreadSummary
	err = db.Table("threads").
		Where("forum_id = ? AND approved = ?", id, true).
		Order("created_at DESC").
		Limit(20).
		Select("id, forum_id, title, content, user_name, user_type, approved as is_approved, created_at, updated_at").
		Find(&threads).Error

	if err != nil {
		return nil, err
	}

	return &ForumWithThreads{
		Forum:   *forum,
		Threads: threads,
	}, nil
}

// CreateInput defines the payload to create a forum.
type CreateInput struct {
	SubscriptionID   uuid.UUID
	Title            string
	Description      *string
	AssistantsOnly   *bool
	RequiresApproval *bool
	Active           *bool
	Order            *int
}

// Create inserts a new forum.
func Create(db *gorm.DB, input CreateInput) (*Forum, error) {
	if input.Title == "" {
		return nil, ErrTitleRequired
	}

	// Check for existing forum with same title
	trimmedTitle := strings.TrimSpace(input.Title)
	var existing Forum
	err := db.Where("subscription_id = ? AND LOWER(title) = ? AND active = ?",
		input.SubscriptionID, strings.ToLower(trimmedTitle), true).
		First(&existing).Error

	if err == nil {
		return nil, ErrTitleExists
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	forum := Forum{
		SubscriptionID: input.SubscriptionID,
		Title:          trimmedTitle,
		Description:    input.Description,
	}

	if input.Description != nil {
		trimmedDesc := strings.TrimSpace(*input.Description)
		forum.Description = &trimmedDesc
	}

	if input.AssistantsOnly != nil {
		forum.AssistantsOnly = *input.AssistantsOnly
	}
	if input.RequiresApproval != nil {
		forum.RequiresApproval = *input.RequiresApproval
	}
	if input.Active != nil {
		forum.Active = *input.Active
	}
	if input.Order != nil {
		forum.Order = *input.Order
	}

	if err := db.Create(&forum).Error; err != nil {
		return nil, err
	}

	return &forum, nil
}

// UpdateInput defines the payload to update a forum.
type UpdateInput struct {
	Title               *string
	DescriptionProvided bool
	Description         *string
	AssistantsOnly      *bool
	RequiresApproval    *bool
	Active              *bool
	OrderProvided       bool
	Order               *int
}

// Update modifies an existing forum.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (*Forum, error) {
	var forum Forum
	if err := db.First(&forum, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrForumNotFound
		}
		return nil, err
	}

	updates := map[string]interface{}{}

	if input.Title != nil {
		trimmedTitle := strings.TrimSpace(*input.Title)
		if trimmedTitle == "" {
			return nil, ErrTitleRequired
		}

		// Check for existing forum with same title (excluding current forum)
		if !strings.EqualFold(trimmedTitle, forum.Title) {
			var existing Forum
			err := db.Where("subscription_id = ? AND LOWER(title) = ? AND active = ? AND id != ?",
				forum.SubscriptionID, strings.ToLower(trimmedTitle), true, id).
				First(&existing).Error

			if err == nil {
				return nil, ErrTitleExists
			}
			if err != gorm.ErrRecordNotFound {
				return nil, err
			}
		}

		updates["title"] = trimmedTitle
	}

	if input.DescriptionProvided {
		if input.Description != nil {
			trimmedDesc := strings.TrimSpace(*input.Description)
			updates["description"] = &trimmedDesc
		} else {
			updates["description"] = nil
		}
	}

	if input.AssistantsOnly != nil {
		updates["assistants_only"] = *input.AssistantsOnly
	}

	if input.RequiresApproval != nil {
		updates["requires_approval"] = *input.RequiresApproval
	}

	if input.Active != nil {
		updates["active"] = *input.Active
	}

	if input.OrderProvided {
		if input.Order != nil {
			updates["order"] = *input.Order
		} else {
			updates["order"] = 0
		}
	}

	if len(updates) > 0 {
		if err := db.Model(&forum).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	// Reload to get updated values
	if err := db.First(&forum, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &forum, nil
}

// Delete removes a forum and its threads.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Forum{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrForumNotFound
	}

	return nil
}
