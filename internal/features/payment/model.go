package payment

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Payment represents a financial transaction.
type Payment struct {
	types.BaseModel

	SubscriptionID       uuid.UUID           `gorm:"type:uuid;not null;column:subscription_id;index:idx_subscription_date" json:"subscriptionId"`
	PaymentMethod        types.PaymentMethod `gorm:"type:varchar(50);not null;default:'Cash';column:payment_method" json:"paymentMethod"`
	ScreenshotURL        *string             `gorm:"type:text;column:screenshot_url" json:"screenshotUrl,omitempty"`
	TransactionReference *string             `gorm:"type:varchar(255);column:transaction_reference" json:"transactionReference,omitempty"`
	Details              *string             `gorm:"type:text" json:"details,omitempty"`
	SubscriptionPoints   int                 `gorm:"type:int;not null;column:subscription_points" json:"subscriptionPoints"`
	Amount               types.Money         `gorm:"type:numeric(10,2);not null" json:"amount"`
	RefundedAmount       types.Money         `gorm:"type:numeric(10,2);not null;default:0;column:refunded_amount" json:"refundedAmount"`
	Discount             types.Money         `gorm:"type:numeric(10,2);not null;default:0" json:"discount"`
	PeriodInDays         int                 `gorm:"type:int;not null;column:period_in_days" json:"periodInDays"`
	IsAddition           bool                `gorm:"type:boolean;not null;default:false;column:is_addition" json:"isAddition"`
	Date                 time.Time           `gorm:"type:timestamp;not null;default:now();index:idx_subscription_date" json:"date"`
	Currency             types.Currency      `gorm:"type:varchar(3);not null;default:'EGP'" json:"currency"`
	Status               types.PaymentStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
}

// TableName overrides the default table name.
func (Payment) TableName() string { return "payments" }

// ListFilters defines payment query filters.
type ListFilters struct {
	SubscriptionID *uuid.UUID
	Keyword        string
	PaymentMethod  string
	Status         string
	DateFrom       *time.Time
	DateTo         *time.Time
	SortBy         string
	SortOrder      string
}

// CreateInput carries data for creating a new payment.
type CreateInput struct {
	SubscriptionID       uuid.UUID
	PaymentMethod        *types.PaymentMethod
	ScreenshotURL        *string
	TransactionReference *string
	Details              *string
	SubscriptionPoints   int
	Amount               types.Money
	RefundedAmount       *types.Money
	Discount             *types.Money
	PeriodInDays         int
	IsAddition           *bool
	Date                 *time.Time
	Currency             *types.Currency
	Status               *types.PaymentStatus
}

// UpdateInput captures mutable payment fields.
type UpdateInput struct {
	PaymentMethod         *types.PaymentMethod
	ScreenshotURLProvided bool
	ScreenshotURL         *string
	TransactionReference  *string
	TransactionProvided   bool
	Details               *string
	DetailsProvided       bool
	SubscriptionPoints    *int
	Amount                *types.Money
	RefundedAmount        *types.Money
	Discount              *types.Money
	PeriodInDays          *int
	IsAddition            *bool
	Date                  *time.Time
	Currency              *types.Currency
	Status                *types.PaymentStatus
}

// List retrieves paginated payments with filters.
func List(db *gorm.DB, filters ListFilters, params pagination.Params) ([]Payment, int64, error) {
	query := db.Model(&Payment{})

	if filters.SubscriptionID != nil {
		query = query.Where("subscription_id = ?", *filters.SubscriptionID)
	}

	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}

	if filters.PaymentMethod != "" {
		query = query.Where("payment_method = ?", filters.PaymentMethod)
	}

	if filters.DateFrom != nil {
		query = query.Where("date >= ?", *filters.DateFrom)
	}

	if filters.DateTo != nil {
		query = query.Where("date <= ?", *filters.DateTo)
	}

	if filters.Keyword != "" {
		keyword := "%" + strings.ToLower(filters.Keyword) + "%"
		query = query.Where("LOWER(details) LIKE ? OR LOWER(transaction_reference) LIKE ?", keyword, keyword)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sorting
	sortColumn := "date"
	sortOrder := "DESC"

	validSortColumns := map[string]bool{
		"date":               true,
		"amount":             true,
		"status":             true,
		"createdAt":          true,
		"subscriptionPoints": true,
	}

	if filters.SortBy != "" && validSortColumns[filters.SortBy] {
		sortColumn = filters.SortBy
	}

	if strings.ToUpper(filters.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}

	var payments []Payment
	err := query.
		Order(sortColumn + " " + sortOrder).
		Offset(params.Skip).
		Limit(params.Limit).
		Find(&payments).Error

	return payments, total, err
}

// Get retrieves a payment by ID.
func Get(db *gorm.DB, id uuid.UUID) (Payment, error) {
	var payment Payment
	if err := db.First(&payment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return payment, ErrPaymentNotFound
		}
		return payment, err
	}
	return payment, nil
}

// Create inserts a new payment.
func Create(db *gorm.DB, input CreateInput) (Payment, error) {
	// Default payment method
	paymentMethod := types.PaymentMethodCash
	if input.PaymentMethod != nil {
		// Validate payment method
		validMethod := false
		for _, method := range ValidPaymentMethods() {
			if *input.PaymentMethod == method {
				validMethod = true
				break
			}
		}
		if !validMethod {
			return Payment{}, ErrInvalidPaymentMethod
		}
		paymentMethod = *input.PaymentMethod
	}

	date := time.Now()
	if input.Date != nil {
		date = *input.Date
	}

	status := types.PaymentStatusPending
	if input.Status != nil {
		// Validate status
		validStatus := false
		for _, s := range ValidStatuses() {
			if *input.Status == s {
				validStatus = true
				break
			}
		}
		if !validStatus {
			return Payment{}, ErrInvalidStatus
		}
		status = *input.Status
	}

	// Default currency
	currency := types.CurrencyEGP
	if input.Currency != nil {
		currency = *input.Currency
	}

	// Default refundedAmount
	refundedAmount := types.NewMoney(0)
	if input.RefundedAmount != nil {
		refundedAmount = *input.RefundedAmount
	}

	// Default discount
	discount := types.NewMoney(0)
	if input.Discount != nil {
		discount = *input.Discount
	}

	// Default isAddition
	isAddition := false
	if input.IsAddition != nil {
		isAddition = *input.IsAddition
	}

	payment := Payment{
		SubscriptionID:       input.SubscriptionID,
		Date:                 date,
		Amount:               input.Amount,
		PaymentMethod:        paymentMethod,
		ScreenshotURL:        input.ScreenshotURL,
		Details:              input.Details,
		TransactionReference: input.TransactionReference,
		Status:               status,
		SubscriptionPoints:   input.SubscriptionPoints,
		RefundedAmount:       refundedAmount,
		Discount:             discount,
		PeriodInDays:         input.PeriodInDays,
		IsAddition:           isAddition,
		Currency:             currency,
	}

	if err := db.Create(&payment).Error; err != nil {
		return Payment{}, err
	}

	return payment, nil
}

// Update modifies an existing payment.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (Payment, error) {
	payment, err := Get(db, id)
	if err != nil {
		return payment, err
	}

	if input.Date != nil {
		payment.Date = *input.Date
	}

	if input.Amount != nil {
		payment.Amount = *input.Amount
	}

	if input.PaymentMethod != nil {
		// Validate payment method
		validMethod := false
		for _, method := range ValidPaymentMethods() {
			if *input.PaymentMethod == method {
				validMethod = true
				break
			}
		}
		if !validMethod {
			return payment, ErrInvalidPaymentMethod
		}
		payment.PaymentMethod = *input.PaymentMethod
	}

	if input.ScreenshotURLProvided {
		payment.ScreenshotURL = input.ScreenshotURL
	}

	if input.DetailsProvided {
		payment.Details = input.Details
	}

	if input.TransactionProvided {
		payment.TransactionReference = input.TransactionReference
	}

	if input.Status != nil {
		// Validate status
		validStatus := false
		for _, s := range ValidStatuses() {
			if *input.Status == s {
				validStatus = true
				break
			}
		}
		if !validStatus {
			return payment, ErrInvalidStatus
		}
		payment.Status = *input.Status
	}

	if input.SubscriptionPoints != nil {
		payment.SubscriptionPoints = *input.SubscriptionPoints
	}

	if input.RefundedAmount != nil {
		payment.RefundedAmount = *input.RefundedAmount
	}

	if input.Discount != nil {
		payment.Discount = *input.Discount
	}

	if input.PeriodInDays != nil {
		payment.PeriodInDays = *input.PeriodInDays
	}

	if input.IsAddition != nil {
		payment.IsAddition = *input.IsAddition
	}

	if input.Currency != nil {
		payment.Currency = *input.Currency
	}

	if err := db.Save(&payment).Error; err != nil {
		return payment, err
	}

	return payment, nil
}

// Delete removes a payment.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Payment{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPaymentNotFound
	}
	return nil
}
