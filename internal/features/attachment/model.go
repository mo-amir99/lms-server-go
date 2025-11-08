package attachment

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Attachment represents a lesson attachment (file or link).
type Attachment struct {
	types.BaseModel

	LessonID  uuid.UUID  `gorm:"type:uuid;not null;column:lesson_id;index:idx_lesson_active,idx_lesson_order" json:"lessonId"`
	Name      string     `gorm:"type:varchar(50);not null" json:"name"`
	Type      string     `gorm:"type:varchar(50);not null;index" json:"type"`
	Path      *string    `gorm:"type:text" json:"path,omitempty"`
	Order     int        `gorm:"type:int;not null;default:0;index:idx_lesson_order" json:"order"`
	Active    bool       `gorm:"type:boolean;not null;default:true;column:is_active;index:idx_lesson_active" json:"isActive"`
	Questions types.JSON `gorm:"type:jsonb" json:"questions,omitempty"` // JSON array of MCQ questions
}

// TableName overrides the default table name.
func (Attachment) TableName() string { return "attachments" }

// CreateInput carries data for creating a new attachment.
type CreateInput struct {
	LessonID  uuid.UUID
	Name      string
	Type      string
	Path      *string
	Order     *int
	Active    *bool
	Questions *types.JSON
}

// UpdateInput captures mutable attachment fields.
type UpdateInput struct {
	Name              *string
	Type              *string
	Path              *string
	PathProvided      bool
	Order             *int
	OrderProvided     bool
	Active            *bool
	Questions         *types.JSON
	QuestionsProvided bool
}

// GetByLesson retrieves all attachments for a lesson.
func GetByLesson(db *gorm.DB, lessonID uuid.UUID) ([]Attachment, error) {
	var attachments []Attachment
	err := db.Where("lesson_id = ?", lessonID).
		Order("\"order\" ASC NULLS LAST, name ASC").
		Find(&attachments).Error
	return attachments, err
}

// Get retrieves an attachment by ID.
func Get(db *gorm.DB, id uuid.UUID) (Attachment, error) {
	var attachment Attachment
	if err := db.First(&attachment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return attachment, ErrAttachmentNotFound
		}
		return attachment, err
	}
	return attachment, nil
}

// Create inserts a new attachment.
func Create(db *gorm.DB, input CreateInput) (Attachment, error) {
	if input.Name == "" {
		return Attachment{}, ErrNameRequired
	}

	if input.Type == "" {
		return Attachment{}, ErrTypeRequired
	}

	// Validate type
	validType := false
	for _, t := range ValidTypes() {
		if input.Type == t {
			validType = true
			break
		}
	}
	if !validType {
		return Attachment{}, ErrInvalidType
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}

	order := 0
	if input.Order != nil {
		order = *input.Order
	}

	attachment := Attachment{
		LessonID: input.LessonID,
		Name:     input.Name,
		Type:     input.Type,
		Path:     input.Path,
		Order:    order,
		Active:   active,
	}

	if input.Questions != nil {
		attachment.Questions = *input.Questions
	}

	if err := db.Create(&attachment).Error; err != nil {
		return Attachment{}, err
	}

	return attachment, nil
}

// Update modifies an existing attachment.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (Attachment, error) {
	attachment, err := Get(db, id)
	if err != nil {
		return attachment, err
	}

	if input.Name != nil {
		if *input.Name == "" {
			return attachment, ErrNameRequired
		}
		attachment.Name = *input.Name
	}

	if input.Type != nil {
		// Validate type
		validType := false
		for _, t := range ValidTypes() {
			if *input.Type == t {
				validType = true
				break
			}
		}
		if !validType {
			return attachment, ErrInvalidType
		}
		attachment.Type = *input.Type
	}

	if input.PathProvided {
		attachment.Path = input.Path
	}

	if input.OrderProvided {
		if input.Order != nil {
			attachment.Order = *input.Order
		} else {
			attachment.Order = 0
		}
	}

	if input.Active != nil {
		attachment.Active = *input.Active
	}

	if input.QuestionsProvided {
		if input.Questions == nil {
			attachment.Questions = nil
		} else {
			attachment.Questions = *input.Questions
		}
	}

	if err := db.Save(&attachment).Error; err != nil {
		return attachment, err
	}

	return attachment, nil
}

// Delete removes an attachment.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Attachment{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAttachmentNotFound
	}
	return nil
}
