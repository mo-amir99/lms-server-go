package groupaccess

import (
	"errors"
	"net/http"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	pkg "github.com/mo-amir99/lms-server-go/internal/features/package"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/pkg/response"
)

// Handler processes group access HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a group access handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// Create creates a new group access with points validation.
func (h *Handler) Create(c *gin.Context) {
	subscriptionID := c.Param("subscriptionId")

	var req struct {
		Name          string   `json:"name" binding:"required"`
		Users         []string `json:"users"`
		Courses       []string `json:"courses"`
		Lessons       []string `json:"lessons"`
		Announcements []string `json:"announcements"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid group access payload", err)
		return
	}

	subID, err := uuid.Parse(subscriptionID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription ID", err)
		return
	}

	// Get subscription to check points limit
	var sub subscription.Subscription
	if err := h.db.First(&sub, "id = ?", subscriptionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Subscription not found", nil)
			return
		}
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to fetch subscription", err)
		return
	}

	// Create group and calculate points
	group := &GroupAccess{
		SubscriptionID: subID,
		Name:           req.Name,
		Users:          req.Users,
		Courses:        req.Courses,
		Lessons:        req.Lessons,
		Announcements:  req.Announcements,
	}

	points, err := group.CalculatePoints(h.db)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to calculate points", err)
		return
	}
	group.SubscriptionPointsUsage = points

	// Check current total usage
	var currentUsage int64
	h.db.Model(&GroupAccess{}).
		Where("subscription_id = ?", subscriptionID).
		Select("COALESCE(SUM(subscription_points_usage), 0)").
		Scan(&currentUsage)

	// Get available points from subscription or package
	availablePoints := sub.SubscriptionPoints
	if sub.PackageID != nil && availablePoints == 0 {
		var packageModel pkg.Package
		if err := h.db.First(&packageModel, "id = ?", sub.PackageID).Error; err == nil {
			if packageModel.SubscriptionPoints != nil {
				availablePoints = *packageModel.SubscriptionPoints
			}
		}
	}

	newUsage := int(currentUsage) + points
	if newUsage > availablePoints {
		response.Error(c, http.StatusBadRequest,
			"Subscription points limit exceeded",
			gin.H{
				"available":      availablePoints,
				"currentUsage":   currentUsage,
				"requiredPoints": points,
				"wouldExceedBy":  newUsage - availablePoints,
				"groupDetails": gin.H{
					"users":         len(req.Users),
					"uniqueCourses": len(group.Courses) + len(group.Lessons), // Approximation
					"pointsPerUser": points / max(len(req.Users), 1),
				},
			})
		return
	}

	// Create group
	if err := h.db.Create(group).Error; err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to create group access", err)
		return
	}

	response.Created(c, gin.H{
		"group": group,
		"pointsInfo": gin.H{
			"groupPoints": points,
			"totalUsage":  newUsage,
			"remaining":   availablePoints - newUsage,
		},
	}, "Group created successfully")
}

// List returns all groups for a subscription.
func (h *Handler) List(c *gin.Context) {
	subscriptionID := c.Param("subscriptionId")

	var groups []GroupAccess
	if err := h.db.Where("subscription_id = ?", subscriptionID).Find(&groups).Error; err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to fetch groups", err)
		return
	}

	response.Success(c, http.StatusOK, groups, "", nil)
}

// Get returns a specific group.
func (h *Handler) Get(c *gin.Context) {
	groupID := c.Param("groupId")

	var group GroupAccess
	if err := h.db.First(&group, "id = ?", groupID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Group not found", nil)
			return
		}
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to fetch group", err)
		return
	}

	response.Success(c, http.StatusOK, group, "", nil)
}

// Update updates a group access with points recalculation.
func (h *Handler) Update(c *gin.Context) {
	groupID := c.Param("groupId")
	subscriptionID := c.Param("subscriptionId")

	var req struct {
		Name          *string   `json:"name"`
		Users         *[]string `json:"users"`
		Courses       *[]string `json:"courses"`
		Lessons       *[]string `json:"lessons"`
		Announcements *[]string `json:"announcements"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid update payload", err)
		return
	}

	var group GroupAccess
	if err := h.db.First(&group, "id = ?", groupID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Group not found", nil)
			return
		}
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to fetch group", err)
		return
	}

	oldPoints := group.SubscriptionPointsUsage

	// Update fields if provided
	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.Users != nil {
		group.Users = *req.Users
	}
	if req.Courses != nil {
		group.Courses = *req.Courses
	}
	if req.Lessons != nil {
		group.Lessons = *req.Lessons
	}
	if req.Announcements != nil {
		group.Announcements = *req.Announcements
	}

	// Recalculate points
	newPoints, err := group.CalculatePoints(h.db)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to calculate points", err)
		return
	}
	group.SubscriptionPointsUsage = newPoints

	// Check points limit
	var sub subscription.Subscription
	if err := h.db.First(&sub, "id = ?", subscriptionID).Error; err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusNotFound, "subscription not found", err)
		return
	}

	// Get current usage excluding this group
	var currentUsage int64
	h.db.Model(&GroupAccess{}).
		Where("subscription_id = ? AND id != ?", subscriptionID, groupID).
		Select("COALESCE(SUM(subscription_points_usage), 0)").
		Scan(&currentUsage)

	availablePoints := sub.SubscriptionPoints
	if sub.PackageID != nil && availablePoints == 0 {
		var packageModel pkg.Package
		if err := h.db.First(&packageModel, "id = ?", sub.PackageID).Error; err == nil {
			if packageModel.SubscriptionPoints != nil {
				availablePoints = *packageModel.SubscriptionPoints
			}
		}
	}

	newUsage := int(currentUsage) + newPoints
	if newUsage > availablePoints {
		response.Error(c, http.StatusBadRequest,
			"Subscription points limit exceeded",
			gin.H{
				"available":      availablePoints,
				"currentUsage":   currentUsage,
				"requiredPoints": newPoints,
				"wouldExceedBy":  newUsage - availablePoints,
			})
		return
	}

	// Save
	if err := h.db.Save(&group).Error; err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to update group", err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"group": group,
		"pointsInfo": gin.H{
			"oldPoints":  oldPoints,
			"newPoints":  newPoints,
			"totalUsage": newUsage,
			"remaining":  availablePoints - newUsage,
		},
	}, "Group updated successfully", nil)
}

// Delete deletes a group access.
func (h *Handler) Delete(c *gin.Context) {
	groupID := c.Param("groupId")

	result := h.db.Delete(&GroupAccess{}, "id = ?", groupID)
	if result.Error != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to delete group", result.Error)
		return
	}

	if result.RowsAffected == 0 {
		response.Error(c, http.StatusNotFound, "Group not found", nil)
		return
	}

	response.Success(c, http.StatusOK, true, "Group deleted successfully", nil)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
