package course

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Course represents an LMS course under a subscription.
type Course struct {
	types.BaseModel

	SubscriptionID   uuid.UUID `gorm:"type:uuid;not null;column:subscription_id;uniqueIndex:idx_subscription_name" json:"subscriptionId"`
	Name             string    `gorm:"type:varchar(100);not null;uniqueIndex:idx_subscription_name" json:"name"`
	Image            *string   `gorm:"type:text" json:"image,omitempty"`
	Description      *string   `gorm:"type:varchar(400)" json:"description,omitempty"`
	CollectionID     *string   `gorm:"type:varchar(255);column:collection_id" json:"collectionId,omitempty"`
	StreamStorageGB  float64   `gorm:"type:numeric(10,2);not null;default:0;column:stream_storage_gb" json:"streamStorageGB"`
	FileStorageGB    float64   `gorm:"type:numeric(10,2);not null;default:0;column:file_storage_gb" json:"fileStorageGB"`
	StorageUsageInGB float64   `gorm:"type:numeric(10,2);not null;default:0;column:storage_usage_in_gb" json:"storageUsageInGB"`
	Order            int       `gorm:"type:int;not null;default:0" json:"order"`
	Active           bool      `gorm:"type:boolean;not null;default:true;column:is_active" json:"isActive"`
}

// TableName overrides the default table name.
func (Course) TableName() string { return "courses" }

// ListFilters defines course query filters.
type ListFilters struct {
	SubscriptionID uuid.UUID
	Keyword        string
	ActiveOnly     bool
}

// CreateInput carries data for creating a new course.
type CreateInput struct {
	SubscriptionID   uuid.UUID
	Name             string
	Image            *string
	Description      *string
	CollectionID     *string
	StreamStorageGB  *float64
	FileStorageGB    *float64
	StorageUsageInGB *float64
	Order            *int
	Active           *bool
}

// UpdateInput captures mutable course fields.
type UpdateInput struct {
	Name             *string
	ImageProvided    bool
	Image            *string
	DescProvided     bool
	Description      *string
	CollIDProvided   bool
	CollectionID     *string
	StreamStorageGB  *float64
	FileStorageGB    *float64
	StorageUsageInGB *float64
	OrderProvided    bool
	Order            *int
	Active           *bool
}

// List retrieves paginated courses with filters.
func List(db *gorm.DB, filters ListFilters, params pagination.Params) ([]Course, int64, error) {
	query := db.Model(&Course{}).Where("subscription_id = ?", filters.SubscriptionID)

	if filters.Keyword != "" {
		keyword := "%" + strings.ToLower(filters.Keyword) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", keyword, keyword)
	}

	if filters.ActiveOnly {
		query = query.Where("is_active = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var courses []Course
	err := query.
		Order("\"order\" ASC NULLS LAST, name ASC").
		Offset(params.Skip).
		Limit(params.Limit).
		Find(&courses).Error

	return courses, total, err
}

// Get retrieves a course by ID.
func Get(db *gorm.DB, id uuid.UUID) (Course, error) {
	var course Course
	if err := db.First(&course, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return course, ErrCourseNotFound
		}
		return course, err
	}
	return course, nil
}

// GetForSubscription retrieves a course by ID that belongs to the provided subscription.
func GetForSubscription(db *gorm.DB, id, subscriptionID uuid.UUID) (Course, error) {
	var course Course
	if err := db.First(&course, "id = ? AND subscription_id = ?", id, subscriptionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return course, ErrCourseNotFound
		}
		return course, err
	}
	return course, nil
}

// Create inserts a new course.
func Create(db *gorm.DB, input CreateInput) (Course, error) {
	if input.Name == "" {
		return Course{}, ErrNameRequired
	}

	// Check order uniqueness if provided
	if input.Order != nil {
		var existing Course
		err := db.First(&existing, "subscription_id = ? AND \"order\" = ?", input.SubscriptionID, *input.Order).Error
		if err == nil {
			return Course{}, ErrOrderTaken
		}
		if err != gorm.ErrRecordNotFound {
			return Course{}, err
		}
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}

	order := 0
	if input.Order != nil {
		order = *input.Order
	}

	course := Course{
		SubscriptionID: input.SubscriptionID,
		Name:           input.Name,
		Image:          input.Image,
		Description:    input.Description,
		CollectionID:   input.CollectionID,
		Order:          order,
		Active:         active,
	}

	if input.StreamStorageGB != nil {
		course.StreamStorageGB = *input.StreamStorageGB
	}
	if input.FileStorageGB != nil {
		course.FileStorageGB = *input.FileStorageGB
	}
	if input.StorageUsageInGB != nil {
		course.StorageUsageInGB = *input.StorageUsageInGB
	}

	if err := db.Create(&course).Error; err != nil {
		return Course{}, err
	}

	return course, nil
}

// Update modifies an existing course.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (Course, error) {
	course, err := Get(db, id)
	if err != nil {
		return course, err
	}

	if input.Name != nil {
		if *input.Name == "" {
			return course, ErrNameRequired
		}
		course.Name = *input.Name
	}

	if input.DescProvided {
		course.Description = input.Description
	}

	if input.OrderProvided {
		// Check order uniqueness if changing
		if input.Order != nil {
			var existing Course
			err := db.First(&existing, "subscription_id = ? AND \"order\" = ? AND id != ?", course.SubscriptionID, *input.Order, id).Error
			if err == nil {
				return course, ErrOrderTaken
			}
			if err != gorm.ErrRecordNotFound {
				return course, err
			}
			course.Order = *input.Order
		} else {
			course.Order = 0
		}
	}

	if input.Active != nil {
		course.Active = *input.Active
	}

	if input.ImageProvided {
		course.Image = input.Image
	}

	if input.CollIDProvided {
		course.CollectionID = input.CollectionID
	}

	if input.StreamStorageGB != nil {
		course.StreamStorageGB = *input.StreamStorageGB
	}
	if input.FileStorageGB != nil {
		course.FileStorageGB = *input.FileStorageGB
	}
	if input.StorageUsageInGB != nil {
		course.StorageUsageInGB = *input.StorageUsageInGB
	}

	if err := db.Save(&course).Error; err != nil {
		return course, err
	}

	return course, nil
}

// Delete removes a course.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Course{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCourseNotFound
	}
	return nil
}

// GetBySubscription retrieves all courses for a subscription.
func GetBySubscription(db *gorm.DB, subscriptionID uuid.UUID) ([]Course, error) {
	var courses []Course
	err := db.Where("subscription_id = ?", subscriptionID).
		Order("\"order\" ASC NULLS LAST, name ASC").
		Find(&courses).Error
	return courses, err
}
