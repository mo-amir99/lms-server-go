package supportticket

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// SupportTicket represents a customer support ticket.
type SupportTicket struct {
	types.BaseModel

	UserID         uuid.UUID `gorm:"type:uuid;not null;index:idx_user_created" json:"userId"`
	SubscriptionID uuid.UUID `gorm:"type:uuid;not null;index:idx_subscription_user,idx_subscription_created" json:"subscriptionId"`
	Subject        string    `gorm:"size:255;not null" json:"subject"`
	Message        string    `gorm:"type:text;not null" json:"message"`
	ReplyInfo      *string   `gorm:"type:text" json:"replyInfo,omitempty"`

	// Association - reference to User model
	User *user.User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

// TableName overrides the default table name.
func (SupportTicket) TableName() string {
	return "support_tickets"
}

// GetBySubscription retrieves all tickets for a subscription with user info.
func GetBySubscription(db *gorm.DB, subscriptionID uuid.UUID) ([]SupportTicket, error) {
	var tickets []SupportTicket
	if err := db.Where("subscription_id = ?", subscriptionID).
		Preload("User").
		Order("created_at DESC").
		Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

// GetByUserAndSubscription retrieves all tickets for a specific user in a subscription with user info.
func GetByUserAndSubscription(db *gorm.DB, userID, subscriptionID uuid.UUID) ([]SupportTicket, error) {
	var tickets []SupportTicket
	if err := db.Where("user_id = ? AND subscription_id = ?", userID, subscriptionID).
		Preload("User").
		Order("created_at DESC").
		Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

// Get retrieves a single ticket by ID with user info.
func Get(db *gorm.DB, id uuid.UUID) (*SupportTicket, error) {
	var ticket SupportTicket
	if err := db.Preload("User").
		First(&ticket, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

// CreateInput defines the payload to create a ticket.
type CreateInput struct {
	UserID         uuid.UUID
	SubscriptionID uuid.UUID
	Subject        string
	Message        string
	ReplyInfo      *string
}

// Create inserts a new support ticket.
func Create(db *gorm.DB, input CreateInput) (*SupportTicket, error) {
	if input.Subject == "" {
		return nil, ErrSubjectRequired
	}
	if input.Message == "" {
		return nil, ErrMessageRequired
	}

	ticket := SupportTicket{
		UserID:         input.UserID,
		SubscriptionID: input.SubscriptionID,
		Subject:        input.Subject,
		Message:        input.Message,
		ReplyInfo:      input.ReplyInfo,
	}

	if err := db.Create(&ticket).Error; err != nil {
		return nil, err
	}

	return &ticket, nil
}

// UpdateInput defines the payload to update a ticket.
type UpdateInput struct {
	ReplyInfoProvided bool
	ReplyInfo         *string
}

// Update modifies an existing ticket and returns it with user info.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (*SupportTicket, error) {
	var ticket SupportTicket
	if err := db.First(&ticket, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}

	updates := map[string]interface{}{}

	if input.ReplyInfoProvided {
		updates["reply_info"] = input.ReplyInfo
	}

	if len(updates) > 0 {
		if err := db.Model(&ticket).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	// Reload with user info
	if err := db.Preload("User").First(&ticket, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &ticket, nil
}

// Delete removes a ticket.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&SupportTicket{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrTicketNotFound
	}

	return nil
}
