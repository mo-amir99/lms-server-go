package groupaccess

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// GroupAccess represents access control groups for subscriptions.
type GroupAccess struct {
	types.BaseModel

	SubscriptionID          uuid.UUID      `gorm:"type:uuid;not null;column:subscription_id;index" json:"subscriptionId"`
	Name                    string         `gorm:"type:varchar(100);not null" json:"name"`
	Users                   pq.StringArray `gorm:"type:uuid[];not null;default:'{}';column:users;index" json:"users"`
	Courses                 pq.StringArray `gorm:"type:uuid[];not null;default:'{}';column:courses;index" json:"courses"`
	Lessons                 pq.StringArray `gorm:"type:uuid[];not null;default:'{}';column:lessons;index" json:"lessons"`
	Announcements           pq.StringArray `gorm:"type:uuid[];not null;default:'{}';column:announcements" json:"announcements"`
	SubscriptionPointsUsage int            `gorm:"type:int;not null;default:0;column:subscription_points_usage" json:"subscriptionPointsUsage"`
}

// TableName overrides the default table name.
func (GroupAccess) TableName() string { return "group_access" }

// CalculatePoints computes subscription points: users.length × uniqueCourses.length
func (g *GroupAccess) CalculatePoints(db *gorm.DB) (int, error) {
	userCount := len(g.Users)
	if userCount == 0 {
		return 0, nil
	}

	// Get unique courses: direct courses + courses from lessons
	uniqueCourses := make(map[string]bool)

	// Add direct courses
	for _, courseID := range g.Courses {
		uniqueCourses[courseID] = true
	}

	// Add courses from lessons
	if len(g.Lessons) > 0 {
		// Convert pq.StringArray to []interface{} for GORM IN clause
		lessonIDs := make([]interface{}, len(g.Lessons))
		for i, id := range g.Lessons {
			lessonIDs[i] = id
		}

		var lessonCourses []string
		err := db.Table("lessons").
			Where("id IN ?", lessonIDs).
			Pluck("course_id", &lessonCourses).Error
		if err != nil {
			return 0, err
		}
		for _, courseID := range lessonCourses {
			uniqueCourses[courseID] = true
		}
	}

	points := userCount * len(uniqueCourses)
	return points, nil
}
