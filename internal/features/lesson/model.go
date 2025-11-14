package lesson

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/attachment"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Lesson represents a lesson within a course.
type Lesson struct {
	types.BaseModel

	CourseID        uuid.UUID      `gorm:"type:uuid;not null;column:course_id" json:"courseId"`
	VideoID         string         `gorm:"type:varchar(255);not null;column:video_id" json:"videoId"`
	ProcessingJobID *string        `gorm:"type:varchar(255);column:processing_job_id;index" json:"processingJobId,omitempty"`
	Name            string         `gorm:"type:varchar(80);not null" json:"name"`
	Description     *string        `gorm:"type:varchar(1000)" json:"description,omitempty"`
	Duration        int            `gorm:"type:int;not null;default:0" json:"duration"` // seconds
	Order           int            `gorm:"type:int;not null;default:0" json:"order"`
	Active          bool           `gorm:"type:boolean;not null;default:true;column:is_active" json:"isActive"`
	AttachmentIDs   pq.StringArray `gorm:"type:uuid[];column:attachments" json:"attachmentOrder,omitempty"`

	Attachments []attachment.Attachment `gorm:"foreignKey:LessonID" json:"attachments,omitempty"`
}

// TableName overrides the default table name.
func (Lesson) TableName() string { return "lessons" }

// ListFilters defines lesson query filters.
type ListFilters struct {
	CourseID   uuid.UUID
	Keyword    string
	ActiveOnly bool
}

// CreateInput carries data for creating a new lesson.
type CreateInput struct {
	CourseID        uuid.UUID
	VideoID         string
	ProcessingJobID *string
	Name            string
	Description     *string
	Duration        *int
	Order           *int
	Active          *bool
}

// UpdateInput captures mutable lesson fields.
type UpdateInput struct {
	Name                    *string
	Description             *string
	DescProvided            bool
	ProcessingJobIDProvided bool
	ProcessingJobID         *string
	Duration                *int
	OrderProvided           bool
	Order                   *int
	VideoIDProvided         bool
	VideoID                 *string
	Active                  *bool
	AttachmentsProvided     bool
	Attachments             []string
}

// List retrieves paginated lessons with filters.
func List(db *gorm.DB, filters ListFilters, params pagination.Params) ([]Lesson, int64, error) {
	query := db.Model(&Lesson{}).Where("course_id = ?", filters.CourseID)

	if filters.Keyword != "" {
		keyword := "%" + strings.ToLower(filters.Keyword) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", keyword, keyword)
	}

	if filters.ActiveOnly {
		query = query.Where("is_active = ?", true)
	}

	var total int64
	countQuery := db.Model(&Lesson{}).Where("course_id = ?", filters.CourseID)
	if filters.Keyword != "" {
		keyword := "%" + strings.ToLower(filters.Keyword) + "%"
		countQuery = countQuery.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", keyword, keyword)
	}
	if filters.ActiveOnly {
		countQuery = countQuery.Where("is_active = ?", true)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, total, err
	}

	var lessons []Lesson
	err := query.
		Preload("Attachments", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "lesson_id", "name", "type", "path", "\"order\"", "is_active", "created_at", "updated_at").
				Order("\"order\" ASC NULLS LAST, name ASC")
		}).
		Order("\"order\" ASC NULLS LAST, name ASC").
		Offset(params.Skip).
		Limit(params.Limit).
		Find(&lessons).Error

	if err != nil {
		return lessons, total, err
	}

	for i := range lessons {
		applyAttachmentOrder(&lessons[i])
	}

	return lessons, total, nil
}

// Get retrieves a lesson by ID.
func Get(db *gorm.DB, id uuid.UUID) (Lesson, error) {
	var lesson Lesson
	if err := db.First(&lesson, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return lesson, ErrLessonNotFound
		}
		return lesson, err
	}
	applyAttachmentOrder(&lesson)

	return lesson, nil
}

// GetWithAttachments retrieves a lesson and preloads attachments.
func GetWithAttachments(db *gorm.DB, id uuid.UUID) (Lesson, error) {
	var lesson Lesson
	err := db.Preload("Attachments", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "lesson_id", "name", "type", "path", "\"order\"", "is_active", "created_at", "updated_at").
			Order("\"order\" ASC NULLS LAST, name ASC")
	}).First(&lesson, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return lesson, ErrLessonNotFound
		}
		return lesson, err
	}
	applyAttachmentOrder(&lesson)

	return lesson, nil
}

// Create inserts a new lesson.
func Create(db *gorm.DB, input CreateInput) (Lesson, error) {
	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" {
		return Lesson{}, ErrNameRequired
	}
	if nameLen := utf8.RuneCountInString(trimmedName); nameLen < 3 || nameLen > 80 {
		return Lesson{}, ErrNameLength
	}

	trimmedVideoID := strings.TrimSpace(input.VideoID)
	if trimmedVideoID == "" {
		return Lesson{}, ErrVideoIDRequired
	}

	var description *string
	if input.Description != nil {
		desc := strings.TrimSpace(*input.Description)
		if utf8.RuneCountInString(desc) > 1000 {
			return Lesson{}, ErrDescriptionTooLong
		}
		description = stringPtr(desc)
	}

	var processingJobID *string
	if input.ProcessingJobID != nil {
		job := strings.TrimSpace(*input.ProcessingJobID)
		if job != "" {
			processingJobID = stringPtr(job)
		}
	}

	if input.Order != nil && *input.Order < 0 {
		return Lesson{}, ErrOrderInvalid
	}

	if input.Duration != nil && *input.Duration < 0 {
		return Lesson{}, ErrDurationInvalid
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}

	order := 0
	if input.Order != nil {
		order = *input.Order
	}

	duration := 0
	if input.Duration != nil {
		duration = *input.Duration
	}

	lesson := Lesson{
		CourseID:        input.CourseID,
		VideoID:         trimmedVideoID,
		ProcessingJobID: processingJobID,
		Name:            trimmedName,
		Description:     description,
		Duration:        duration,
		Order:           order,
		Active:          active,
		AttachmentIDs:   pq.StringArray{},
	}

	if err := db.Create(&lesson).Error; err != nil {
		return Lesson{}, err
	}

	return lesson, nil
}

// Update modifies an existing lesson.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (Lesson, error) {
	lesson, err := Get(db, id)
	if err != nil {
		return lesson, err
	}

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			return lesson, ErrNameRequired
		}
		if nameLen := utf8.RuneCountInString(trimmed); nameLen < 3 || nameLen > 80 {
			return lesson, ErrNameLength
		}
		lesson.Name = trimmed
	}

	if input.DescProvided {
		if input.Description == nil {
			lesson.Description = nil
		} else {
			trimmed := strings.TrimSpace(*input.Description)
			if utf8.RuneCountInString(trimmed) > 1000 {
				return lesson, ErrDescriptionTooLong
			}
			lesson.Description = stringPtr(trimmed)
		}
	}

	if input.OrderProvided {
		if input.Order != nil {
			if *input.Order < 0 {
				return lesson, ErrOrderInvalid
			}
			lesson.Order = *input.Order
		} else {
			lesson.Order = 0
		}
	}

	if input.Active != nil {
		lesson.Active = *input.Active
	}

	if input.VideoIDProvided {
		if input.VideoID == nil {
			return lesson, ErrVideoIDRequired
		}
		trimmed := strings.TrimSpace(*input.VideoID)
		if trimmed == "" {
			return lesson, ErrVideoIDRequired
		}
		lesson.VideoID = trimmed
	}

	if input.Duration != nil {
		if *input.Duration < 0 {
			return lesson, ErrDurationInvalid
		}
		lesson.Duration = *input.Duration
	}

	if input.ProcessingJobIDProvided {
		if input.ProcessingJobID == nil {
			lesson.ProcessingJobID = nil
		} else {
			trimmed := strings.TrimSpace(*input.ProcessingJobID)
			if trimmed == "" {
				lesson.ProcessingJobID = nil
			} else {
				lesson.ProcessingJobID = stringPtr(trimmed)
			}
		}
	}

	if input.AttachmentsProvided {
		lesson.AttachmentIDs = pq.StringArray(input.Attachments)
	}

	if err := db.Save(&lesson).Error; err != nil {
		return lesson, err
	}

	return lesson, nil
}

// Delete removes a lesson.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Lesson{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLessonNotFound
	}
	return nil
}

// GetByCourse retrieves all lessons for a course.
func GetByCourse(db *gorm.DB, courseID uuid.UUID) ([]Lesson, error) {
	var lessons []Lesson
	err := db.Where("course_id = ?", courseID).
		Order("\"order\" ASC NULLS LAST, name ASC").
		Find(&lessons).Error
	return lessons, err
}

// AppendAttachmentID appends an attachment ID to the lesson-level attachment order array.
func AppendAttachmentID(db *gorm.DB, lessonID, attachmentID uuid.UUID) error {
	return db.Exec(`UPDATE lessons SET attachments = array_append(COALESCE(attachments, '{}'::uuid[]), ?) WHERE id = ?`, attachmentID, lessonID).Error
}

// RemoveAttachmentID removes an attachment ID from the lesson-level attachment order array.
func RemoveAttachmentID(db *gorm.DB, lessonID, attachmentID uuid.UUID) error {
	return db.Exec(`UPDATE lessons SET attachments = array_remove(COALESCE(attachments, '{}'::uuid[]), ?) WHERE id = ?`, attachmentID, lessonID).Error
}

func stringPtr(value string) *string {
	v := value
	return &v
}

func applyAttachmentOrder(lesson *Lesson) {
	if lesson == nil {
		return
	}

	if len(lesson.AttachmentIDs) == 0 || len(lesson.Attachments) <= 1 {
		return
	}

	indexByID := make(map[string]int, len(lesson.AttachmentIDs))
	for idx, rawID := range lesson.AttachmentIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		indexByID[strings.ToLower(id)] = idx
	}

	if len(indexByID) == 0 {
		return
	}

	sort.SliceStable(lesson.Attachments, func(i, j int) bool {
		idI := strings.ToLower(lesson.Attachments[i].ID.String())
		idJ := strings.ToLower(lesson.Attachments[j].ID.String())

		posI, okI := indexByID[idI]
		posJ, okJ := indexByID[idJ]

		switch {
		case okI && okJ:
			return posI < posJ
		case okI:
			return true
		case okJ:
			return false
		}

		if lesson.Attachments[i].Order == lesson.Attachments[j].Order {
			return lesson.Attachments[i].Name < lesson.Attachments[j].Name
		}

		return lesson.Attachments[i].Order < lesson.Attachments[j].Order
	})
}
