package comment

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Comment represents a comment on a lesson.
type Comment struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	LessonID  uuid.UUID  `gorm:"type:uuid;not null;column:lesson_id;index:idx_lesson_created,priority:1" json:"lessonId"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;column:user_id" json:"userId"`
	UserName  string     `gorm:"type:varchar(255);not null;column:user_name" json:"userName"`
	UserType  string     `gorm:"type:varchar(20);not null;column:user_type" json:"userType"`
	Content   string     `gorm:"type:text;not null" json:"content"`
	ParentID  *uuid.UUID `gorm:"type:uuid;column:parent_id" json:"parentId,omitempty"`
	CreatedAt time.Time  `gorm:"column:created_at;index:idx_lesson_created,priority:2" json:"createdAt"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName overrides the default table name.
func (Comment) TableName() string { return "comments" }

// CreateInput carries data for creating a new comment.
type CreateInput struct {
	LessonID uuid.UUID
	UserID   uuid.UUID
	UserName string
	UserType types.UserType
	Content  string
	ParentID *uuid.UUID
}

// GetByLesson retrieves all comments for a lesson.
func GetByLesson(db *gorm.DB, lessonID uuid.UUID) ([]Comment, error) {
	var comments []Comment
	err := db.Where("lesson_id = ?", lessonID).
		Order("created_at DESC").
		Find(&comments).Error
	return comments, err
}

// Get retrieves a comment by ID.
func Get(db *gorm.DB, id uuid.UUID) (Comment, error) {
	var comment Comment
	if err := db.First(&comment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return comment, ErrCommentNotFound
		}
		return comment, err
	}
	return comment, nil
}

// Create inserts a new comment.
func Create(db *gorm.DB, input CreateInput) (Comment, error) {
	if input.Content == "" {
		return Comment{}, ErrContentRequired
	}

	comment := Comment{
		LessonID: input.LessonID,
		UserID:   input.UserID,
		UserName: input.UserName,
		UserType: string(input.UserType), // Convert typed enum to string for storage
		Content:  input.Content,
		ParentID: input.ParentID,
	}

	if err := db.Create(&comment).Error; err != nil {
		return Comment{}, err
	}

	return comment, nil
}

// Delete removes a comment and all its children recursively.
func Delete(db *gorm.DB, id, lessonID uuid.UUID) error {
	return deleteWithChildren(db, id, lessonID)
}

// deleteWithChildren recursively deletes a comment and all its child comments.
func deleteWithChildren(db *gorm.DB, id, lessonID uuid.UUID) error {
	// Find all children
	var children []Comment
	if err := db.Where("parent_id = ? AND lesson_id = ?", id, lessonID).Find(&children).Error; err != nil {
		return err
	}

	// Recursively delete children
	for _, child := range children {
		if err := deleteWithChildren(db, child.ID, lessonID); err != nil {
			return err
		}
	}

	// Delete the comment itself
	result := db.Where("id = ? AND lesson_id = ?", id, lessonID).Delete(&Comment{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCommentNotFound
	}

	return nil
}
