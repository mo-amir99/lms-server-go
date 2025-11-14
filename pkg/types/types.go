package types

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// UserType represents user role levels
type UserType string

const (
	UserTypeReferrer   UserType = "referrer"
	UserTypeStudent    UserType = "student"
	UserTypeAssistant  UserType = "assistant"
	UserTypeInstructor UserType = "instructor"
	UserTypeAdmin      UserType = "admin"
	UserTypeSuperAdmin UserType = "superadmin"
	UserTypeAll        UserType = "all"
)

// Legacy aliases kept for backward compatibility with the Node.js codebase terminology.
const (
	UserTypeTeacher UserType = UserTypeInstructor
	UserTypeOwner   UserType = UserTypeAdmin
)

// PaymentStatus represents payment state
type PaymentStatus string

const (
	PaymentStatusPending           PaymentStatus = "pending"
	PaymentStatusCompleted         PaymentStatus = "completed"
	PaymentStatusFailed            PaymentStatus = "failed"
	PaymentStatusRefunded          PaymentStatus = "refunded"
	PaymentStatusPartiallyRefunded PaymentStatus = "partially_refunded"
)

// Currency represents supported currencies
type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyEGP Currency = "EGP"
	CurrencySAR Currency = "SAR"
	CurrencyAED Currency = "AED"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
)

// AttachmentType represents attachment content types
type AttachmentType string

const (
	AttachmentTypePDF   AttachmentType = "pdf"
	AttachmentTypeAudio AttachmentType = "audio"
	AttachmentTypeImage AttachmentType = "image"
	AttachmentTypeMCQ   AttachmentType = "mcq"
	AttachmentTypeLink  AttachmentType = "link"
)

// PaymentMethod represents payment methods
type PaymentMethod string

const (
	PaymentMethodCash         PaymentMethod = "cash"
	PaymentMethodInstapay     PaymentMethod = "instapay"
	PaymentMethodCreditCard   PaymentMethod = "credit_card"
	PaymentMethodCrypto       PaymentMethod = "crypto"
	PaymentMethodMobileWallet PaymentMethod = "mobile_wallet"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
	PaymentMethodGooglePlay   PaymentMethod = "google_play"
	PaymentMethodAppStore     PaymentMethod = "app_store"
	PaymentMethodPayPal       PaymentMethod = "paypal"
	PaymentMethodPayoneer     PaymentMethod = "payoneer"
	PaymentMethodStripe       PaymentMethod = "stripe"
	PaymentMethodOther        PaymentMethod = "other"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TimestampModel contains only timestamp fields (for models with custom IDs)
type TimestampModel struct {
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// Reply represents a thread reply
type Reply struct {
	ID        string    `json:"id"`
	UserName  string    `json:"userName"`
	UserType  UserType  `json:"userType"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// MCQQuestion represents a multiple choice question
type MCQQuestion struct {
	Question      string   `json:"question"`
	Answers       []string `json:"answers"`       // Max 4
	CorrectAnswer string   `json:"correctAnswer"` // A, B, C, or D
}

// Validate validates MCQ structure
func (q *MCQQuestion) Validate() error {
	if len(q.Question) < 3 || len(q.Question) > 500 {
		return errors.New("question must be 3-500 characters")
	}
	if len(q.Answers) < 2 || len(q.Answers) > 4 {
		return errors.New("must have 2-4 answers")
	}
	for _, answer := range q.Answers {
		if len(answer) == 0 || len(answer) > 100 {
			return errors.New("each answer must be 1-100 characters")
		}
	}
	if q.CorrectAnswer != "A" && q.CorrectAnswer != "B" &&
		q.CorrectAnswer != "C" && q.CorrectAnswer != "D" {
		return errors.New("correctAnswer must be A, B, C, or D")
	}
	return nil
}

// Money wraps decimal.Decimal for money values
type Money decimal.Decimal

// NewMoney creates Money from float64
func NewMoney(value float64) Money {
	return Money(decimal.NewFromFloat(value))
}

// NewMoneyFromString creates Money from string
func NewMoneyFromString(value string) (Money, error) {
	d, err := decimal.NewFromString(value)
	if err != nil {
		return Money{}, err
	}
	return Money(d), nil
}

// Float64 returns the float64 representation
func (m Money) Float64() float64 {
	return decimal.Decimal(m).InexactFloat64()
}

// String returns string representation
func (m Money) String() string {
	return decimal.Decimal(m).String()
}

// Add adds two Money values
func (m Money) Add(other Money) Money {
	return Money(decimal.Decimal(m).Add(decimal.Decimal(other)))
}

// Sub subtracts other from m
func (m Money) Sub(other Money) Money {
	return Money(decimal.Decimal(m).Sub(decimal.Decimal(other)))
}

// Mul multiplies Money by a scalar
func (m Money) Mul(scalar float64) Money {
	return Money(decimal.Decimal(m).Mul(decimal.NewFromFloat(scalar)))
}

// GreaterThan returns true if m > other
func (m Money) GreaterThan(other Money) bool {
	return decimal.Decimal(m).GreaterThan(decimal.Decimal(other))
}

// LessThan returns true if m < other
func (m Money) LessThan(other Money) bool {
	return decimal.Decimal(m).LessThan(decimal.Decimal(other))
}

// IsZero returns true if value is zero
func (m Money) IsZero() bool {
	return decimal.Decimal(m).IsZero()
}

// Value implements driver.Valuer for database serialization
func (m Money) Value() (driver.Value, error) {
	return decimal.Decimal(m).Value()
}

// Scan implements sql.Scanner for database deserialization
func (m *Money) Scan(value interface{}) error {
	var d decimal.Decimal
	if err := d.Scan(value); err != nil {
		return err
	}
	*m = Money(d)
	return nil
}

// MarshalJSON implements json.Marshaler
func (m Money) MarshalJSON() ([]byte, error) {
	return decimal.Decimal(m).MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler
func (m *Money) UnmarshalJSON(data []byte) error {
	var d decimal.Decimal
	if err := d.UnmarshalJSON(data); err != nil {
		return err
	}
	*m = Money(d)
	return nil
}

// JSON represents a generic JSON blob stored in the database.
type JSON []byte

// Value implements driver.Valuer for JSON serialization.
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return []byte(j), nil
}

// Scan implements sql.Scanner for JSON deserialization.
func (j *JSON) Scan(value interface{}) error {
	switch v := value.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[:0], v...)
	case string:
		*j = append((*j)[:0], v...)
	default:
		return fmt.Errorf("types.JSON: unsupported scan type %T", value)
	}
	return nil
}

// MarshalJSON passes through the stored JSON.
func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON stores the raw JSON bytes.
func (j *JSON) UnmarshalJSON(data []byte) error {
	if data == nil {
		*j = nil
		return nil
	}
	*j = append((*j)[:0], data...)
	return nil
}
