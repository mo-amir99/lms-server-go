package user

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Email validation regex - allows standard emails and subscription domain format (@identifier)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+$`)

// Handler processes user HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a user handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns paginated users with filters.
func (h *Handler) List(c *gin.Context) {
	params := pagination.Extract(c)
	keyword := c.Query("filterKeyword")
	subscriptionFilter := c.Query("subscription")

	// Get current user from context (set by middleware)
	user, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	filters := ListFilters{
		Keyword: keyword,
	}

	// Role-based filtering logic
	if user.UserType != types.UserTypeSuperAdmin {
		// Non-superadmin users can only see users with lower user types
		requesterIndex := UserTypeIndex(user.UserType)
		if requesterIndex >= 0 {
			allowedTypes := UserTypeOrder[:requesterIndex]
			filters.UserTypes = allowedTypes
		}
	}

	// Subscription filtering
	switch user.UserType {
	case types.UserTypeAdmin, types.UserTypeSuperAdmin:
		// Admin/SuperAdmin can filter by subscription and exclude students by default
		if subscriptionFilter != "" {
			subID, err := uuid.Parse(subscriptionFilter)
			if err == nil {
				filters.SubscriptionID = &subID
			}
		}
		// Exclude students and assistants for admin/superadmin views
		filters.ExcludeUserTypes = []types.UserType{types.UserTypeStudent, types.UserTypeAssistant}
	case types.UserTypeInstructor, types.UserTypeAssistant:
		// Instructor/Assistant can only see users from their subscription
		filters.SubscriptionID = user.SubscriptionID
	}

	users, total, err := List(h.db, filters, params)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to list users", err)
		return
	}

	response.Success(c, http.StatusOK, users, "", pagination.MetadataFrom(total, params))
}

type createRequest struct {
	SubscriptionID *string `json:"subscriptionId"`
	FullName       string  `json:"fullName" binding:"required"`
	Email          string  `json:"email" binding:"required"`
	Phone          *string `json:"phone"`
	Password       string  `json:"password" binding:"required"`
	UserType       string  `json:"userType" binding:"required"`
	Active         *bool   `json:"isActive"`
}

// Create inserts a new user.
func (h *Handler) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user payload", err)
		return
	}

	// Validate email format (allows both standard emails and subscription domain format)
	if !emailRegex.MatchString(req.Email) {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid email format", fmt.Errorf("email must be in valid format"))
		return
	}

	// Get current user from context
	requester, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	targetUserType := types.UserType(req.UserType)

	// Authorization: Prevent creating users with higher or equal user type
	if requester.UserType != types.UserTypeSuperAdmin {
		if !CanManageUserType(requester.UserType, targetUserType) {
			response.ErrorWithLog(h.logger, c, http.StatusForbidden, "You are not authorized to create a user with this user type", nil)
			return
		}
	}

	var subscriptionID *uuid.UUID
	if req.SubscriptionID != nil {
		parsed, err := uuid.Parse(*req.SubscriptionID)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
			return
		}
		subscriptionID = &parsed
	}

	// Set subscription for instructor/assistant
	if requester.UserType == types.UserTypeInstructor || requester.UserType == types.UserTypeAssistant {
		subscriptionID = requester.SubscriptionID

		// Email domain check - MUST end with @{identifierName}
		if requester.Subscription == nil || requester.Subscription.IdentifierName == "" {
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Subscription identifier not found", fmt.Errorf("subscription identifier missing"))
			return
		}

		requiredDomain := "@" + requester.Subscription.IdentifierName
		if !strings.HasSuffix(strings.ToLower(req.Email), strings.ToLower(requiredDomain)) {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Email must end with "+requiredDomain, fmt.Errorf("invalid email domain"))
			return
		}

		// Validate email format before the @ symbol
		emailParts := strings.Split(req.Email, "@")
		if len(emailParts) != 2 || emailParts[0] == "" {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Invalid email format", fmt.Errorf("email must have username before @"))
			return
		}

		// Check subscription limits
		if err := h.checkSubscriptionLimits(subscriptionID, targetUserType, nil); err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusForbidden, err.Error(), err)
			return
		}
	}

	input := CreateInput{
		SubscriptionID: subscriptionID,
		FullName:       req.FullName,
		Email:          req.Email,
		Phone:          req.Phone,
		Password:       req.Password,
		UserType:       targetUserType,
		Active:         req.Active,
	}

	user, err := Create(h.db, input)
	if err != nil {
		h.respondError(c, err, "failed to create user")
		return
	}

	response.Created(c, user, "")
}

// GetByID fetches a single user.
func (h *Handler) GetByID(c *gin.Context) {
	requester, _ := c.Get("user")
	requesterUser := requester.(middleware.User)

	id, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	user, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load user")
		return
	}

	// Authorization: user can only see their own profile, users in their subscription, or any user if admin/superadmin
	isAuthorized := requesterUser.UserType == types.UserTypeAdmin ||
		requesterUser.UserType == types.UserTypeSuperAdmin ||
		requesterUser.ID == user.ID

	if !isAuthorized && (requesterUser.UserType == types.UserTypeInstructor || requesterUser.UserType == types.UserTypeAssistant) {
		// Instructor/Assistant can see users in their subscription
		if requesterUser.SubscriptionID != nil && user.SubscriptionID != nil {
			isAuthorized = *requesterUser.SubscriptionID == *user.SubscriptionID
		}
	}

	if !isAuthorized {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "You are not authorized to get this user", nil)
		return
	}

	response.Success(c, http.StatusOK, user, "", nil)
}

// Update modifies an existing user.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	// Get current user from context
	requester, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Get user to update
	userToUpdate, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load user")
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user payload", err)
		return
	}

	input := UpdateInput{}

	// Check if userType is being changed
	if value, ok := body["userType"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "userType must be a string", err)
			return
		}
		targetUserType := types.UserType(str)

		// Authorization check: prevent updating users with higher userType or updating to higher userType
		isSameUser := requester.ID == id
		if requester.UserType != types.UserTypeSuperAdmin && !isSameUser {
			// Check if requester can manage the current user type
			if !CanManageUserType(requester.UserType, userToUpdate.UserType) {
				response.ErrorWithLog(h.logger, c, http.StatusForbidden, "You are not authorized to update this user", nil)
				return
			}
			// Check if requester can manage the target user type
			if !CanManageUserType(requester.UserType, targetUserType) {
				response.ErrorWithLog(h.logger, c, http.StatusForbidden, "You are not authorized to update this user", nil)
				return
			}
		}

		input.UserType = &targetUserType

		// Check subscription limits if instructor/assistant is changing user type
		if (requester.UserType == types.UserTypeInstructor || requester.UserType == types.UserTypeAssistant) &&
			targetUserType != userToUpdate.UserType {
			if err := h.checkSubscriptionLimits(userToUpdate.SubscriptionID, targetUserType, &id); err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusForbidden, err.Error(), err)
				return
			}
		}
	}

	// Instructors/assistants can only update users in their subscription
	if requester.UserType == types.UserTypeInstructor || requester.UserType == types.UserTypeAssistant {
		if requester.SubscriptionID == nil || userToUpdate.SubscriptionID == nil ||
			*requester.SubscriptionID != *userToUpdate.SubscriptionID {
			response.ErrorWithLog(h.logger, c, http.StatusForbidden, "You are not authorized to update this user", nil)
			return
		}
	}

	if value, ok := body["subscriptionId"]; ok {
		input.SubscriptionIDProvided = true

		// Only admin/superadmin can change subscription
		if requester.UserType != types.UserTypeAdmin && requester.UserType != types.UserTypeSuperAdmin {
			response.ErrorWithLog(h.logger, c, http.StatusForbidden, "Only admins can change subscription", nil)
			return
		}

		if value == nil {
			input.SubscriptionID = nil
		} else {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionId must be a string", err)
				return
			}
			parsed, err := uuid.Parse(str)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
				return
			}
			input.SubscriptionID = &parsed
		}
	}

	if value, ok := body["fullName"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "fullName must be a string", err)
			return
		}
		input.FullName = &str
	}

	if value, ok := body["email"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "email must be a string", err)
			return
		}

		// Validate email format
		if !emailRegex.MatchString(str) {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid email format", fmt.Errorf("email must be in valid format"))
			return
		}

		// Email domain check for instructor/assistant - MUST end with @{identifierName}
		if requester.UserType == types.UserTypeInstructor || requester.UserType == types.UserTypeAssistant {
			if requester.Subscription == nil || requester.Subscription.IdentifierName == "" {
				response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Subscription identifier not found", fmt.Errorf("subscription identifier missing"))
				return
			}

			requiredDomain := "@" + requester.Subscription.IdentifierName
			if !strings.HasSuffix(strings.ToLower(str), strings.ToLower(requiredDomain)) {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Email must end with "+requiredDomain, fmt.Errorf("invalid email domain"))
				return
			}

			// Validate email format before the @ symbol
			emailParts := strings.Split(str, "@")
			if len(emailParts) != 2 || emailParts[0] == "" {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Invalid email format", fmt.Errorf("email must have username before @"))
				return
			}
		}

		input.Email = &str
	}

	if value, ok := body["phone"]; ok {
		input.PhoneProvided = true
		if value == nil {
			input.Phone = nil
		} else {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "phone must be a string", err)
				return
			}
			input.Phone = &str
		}
	}

	if value, ok := body["password"]; ok {
		// Allow null password (means don't update it)
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "password must be a string", err)
				return
			}
			input.Password = &str
		}
	}

	if value, ok := body["isActive"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isActive must be boolean", err)
			return
		}
		input.Active = &val
	}

	user, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update user")
		return
	}

	response.Success(c, http.StatusOK, user, "", nil)
}

// Delete removes a user.
func (h *Handler) Delete(c *gin.Context) {
	requester, _ := c.Get("user")
	requesterUser := requester.(middleware.User)

	id, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	userToDelete, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load user")
		return
	}

	// Authorization rules (matching Node.js implementation):
	// 1. Only superadmin can delete admins or superadmins
	if (userToDelete.UserType == types.UserTypeAdmin || userToDelete.UserType == types.UserTypeSuperAdmin) &&
		requesterUser.UserType != types.UserTypeSuperAdmin {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "Only superadmins can delete admins", nil)
		return
	}

	// 2. Superadmin can delete anyone
	if requesterUser.UserType == types.UserTypeSuperAdmin {
		if err := Delete(h.db, id); err != nil {
			h.respondError(c, err, "failed to delete user")
			return
		}
		response.Success(c, http.StatusOK, true, "", nil)
		return
	}

	// 3. Admin can delete anyone except superadmin
	if requesterUser.UserType == types.UserTypeAdmin {
		if userToDelete.UserType != types.UserTypeSuperAdmin {
			if err := Delete(h.db, id); err != nil {
				h.respondError(c, err, "failed to delete user")
				return
			}
			response.Success(c, http.StatusOK, true, "", nil)
			return
		}
	}

	// 4. Assistants cannot delete instructors or other assistants
	if requesterUser.UserType == types.UserTypeAssistant &&
		(userToDelete.UserType == types.UserTypeInstructor || userToDelete.UserType == types.UserTypeAssistant) {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "Assistants can not delete instructors or other assistants", nil)
		return
	}

	// 5. Instructor/Assistant can delete users in their subscription
	if (requesterUser.UserType == types.UserTypeInstructor || requesterUser.UserType == types.UserTypeAssistant) &&
		requesterUser.SubscriptionID != nil && userToDelete.SubscriptionID != nil &&
		*requesterUser.SubscriptionID == *userToDelete.SubscriptionID {
		if err := Delete(h.db, id); err != nil {
			h.respondError(c, err, "failed to delete user")
			return
		}
		response.Success(c, http.StatusOK, true, "", nil)
		return
	}

	// 6. Student can delete themselves
	if requesterUser.UserType == types.UserTypeStudent && requesterUser.ID == userToDelete.ID {
		if err := Delete(h.db, id); err != nil {
			h.respondError(c, err, "failed to delete user")
			return
		}
		response.Success(c, http.StatusOK, true, "", nil)
		return
	}

	// If none of the above conditions met, deny access
	response.ErrorWithLog(h.logger, c, http.StatusForbidden, "You are not authorized to delete this user", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrUserNotFound):
		status = http.StatusNotFound
		message = "User not found."
	case errors.Is(err, ErrEmailTaken):
		status = http.StatusConflict
		message = "Email already exists."
	case errors.Is(err, ErrInvalidPassword):
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusForbidden
		message = err.Error()
	default:
		if err.Error() == "fullName cannot be empty" || err.Error() == "email cannot be empty" {
			status = http.StatusBadRequest
			message = err.Error()
		}
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}

// checkSubscriptionLimits verifies if the subscription can accommodate a new user of the given type
func (h *Handler) checkSubscriptionLimits(subscriptionID *uuid.UUID, userType types.UserType, excludeUserID *uuid.UUID) error {
	if subscriptionID == nil {
		return nil
	}

	sub, err := subscription.Get(h.db, *subscriptionID)
	if err != nil {
		if errors.Is(err, subscription.ErrSubscriptionNotFound) {
			return fmt.Errorf("subscription not found")
		}
		return fmt.Errorf("failed to load subscription")
	}

	if userType == types.UserTypeStudent {
		studentLimit := int(sub.SubscriptionPoints)
		if studentLimit > 0 {
			var currentStudents int64
			query := h.db.Model(&User{}).Where("subscription_id = ? AND user_type = ?", subscriptionID, types.UserTypeStudent)
			if excludeUserID != nil {
				query = query.Where("id != ?", *excludeUserID)
			}
			if err := query.Count(&currentStudents).Error; err != nil {
				return fmt.Errorf("failed to count students")
			}

			if currentStudents >= int64(studentLimit) {
				return fmt.Errorf("student limit reached for this subscription. Please upgrade to add more students")
			}
		}
	}

	if userType == types.UserTypeAssistant {
		assistantLimit := int(sub.AssistantsLimit)
		if assistantLimit > 0 {
			var currentAssistants int64
			query := h.db.Model(&User{}).Where("subscription_id = ? AND user_type = ?", subscriptionID, types.UserTypeAssistant)
			if excludeUserID != nil {
				query = query.Where("id != ?", *excludeUserID)
			}
			if err := query.Count(&currentAssistants).Error; err != nil {
				return fmt.Errorf("failed to count assistants")
			}

			if currentAssistants >= int64(assistantLimit) {
				return fmt.Errorf("assistant limit reached for this subscription. Please upgrade to add more assistants")
			}
		}
	}

	return nil
}
