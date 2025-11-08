package userwatch

import (
	"time"

	"github.com/google/uuid"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// UserWatch represents a user's watch access to a lesson with an expiration date.
type UserWatch struct {
	types.BaseModel

	UserID   uuid.UUID `gorm:"type:uuid;not null;column:user_id;index" json:"userId"`
	LessonID uuid.UUID `gorm:"type:uuid;not null;column:lesson_id;index" json:"lessonId"`
	EndDate  time.Time `gorm:"type:timestamp;not null;column:end_date;index" json:"endDate"`
}

// TableName overrides the default table name.
func (UserWatch) TableName() string { return "user_watches" }
