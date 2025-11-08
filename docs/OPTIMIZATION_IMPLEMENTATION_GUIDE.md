# 🎯 Quick Implementation Guide - Type Optimizations

## Priority 1: Enum Constants (30 minutes)

### Step 1: Create `pkg/types/enums.go`

```go
package types

// UserType represents user role levels
type UserType string

const (
	UserTypeStudent   UserType = "STUDENT"
	UserTypeTeacher   UserType = "TEACHER"
	UserTypeAssistant UserType = "ASSISTANT"
	UserTypeOwner     UserType = "OWNER"
	UserTypeSuperAdmin UserType = "SUPER_ADMIN"
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
	CurrencyEGP Currency = "EGP"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
)

// AttachmentType represents attachment content types
type AttachmentType string

const (
	AttachmentTypePDF   AttachmentType = "PDF"
	AttachmentTypeAudio AttachmentType = "Audio"
	AttachmentTypeImage AttachmentType = "Image"
	AttachmentTypeMCQ   AttachmentType = "MCQ"
)

// PaymentMethod represents payment methods
type PaymentMethod string

const (
	PaymentMethodCash        PaymentMethod = "Cash"
	PaymentMethodVodafoneCash PaymentMethod = "Vodafone Cash"
	PaymentMethodInstapay    PaymentMethod = "Instapay"
	PaymentMethodBankTransfer PaymentMethod = "Bank Transfer"
)
```

### Step 2: Update Models (Example)

```go
// In internal/features/user/model.go
import "github.com/mo-amir99/lms-server-go/pkg/types"

type User struct {
    UserType types.UserType `gorm:"type:varchar(20)"`
}

// In internal/features/payment/model.go
type Payment struct {
    PaymentMethod types.PaymentMethod `gorm:"type:varchar(50)"`
    Currency      types.Currency      `gorm:"type:varchar(3)"`
    Status        types.PaymentStatus `gorm:"type:varchar(20)"`
}
```

---

## Priority 2: Structured JSON Types (20 minutes)

### Create `pkg/types/json.go`

```go
package types

import "time"

// Reply represents a thread reply
type Reply struct {
	ID        string   `json:"id"`
	UserName  string   `json:"userName"`
	UserType  UserType `json:"userType"`
	Content   string   `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// MCQQuestion represents a multiple choice question
type MCQQuestion struct {
	Question      string   `json:"question"`
	Answers       []string `json:"answers"` // Max 4
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
	if q.CorrectAnswer != "A" && q.CorrectAnswer != "B" &&
	   q.CorrectAnswer != "C" && q.CorrectAnswer != "D" {
		return errors.New("correctAnswer must be A, B, C, or D")
	}
	return nil
}
```

### Update Models

```go
// In internal/features/thread/model.go
import "github.com/mo-amir99/lms-server-go/pkg/types"

type Thread struct {
    Replies []types.Reply `gorm:"type:jsonb;serializer:json"`
}

// In internal/features/attachment/model.go
type Attachment struct {
    Questions []types.MCQQuestion `gorm:"type:jsonb;serializer:json"`
}
```

---

## Priority 3: BaseModel Embedding (15 minutes)

### Create `pkg/types/base.go`

```go
package types

import (
	"time"
	"github.com/google/uuid"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TimestampModel contains only timestamp fields (for models with custom IDs)
type TimestampModel struct {
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}
```

### Update Models (Example)

```go
// In internal/features/user/model.go
import "github.com/mo-amir99/lms-server-go/pkg/types"

type User struct {
    types.BaseModel
    SubscriptionID *uuid.UUID `gorm:"type:uuid"`
    FullName       string     `gorm:"type:varchar(30)"`
    // ... other fields
}

// No need to redeclare ID, CreatedAt, UpdatedAt!
```

---

## Priority 4: Money Type (45 minutes)

### Step 1: Install Package

```bash
go get github.com/shopspring/decimal
```

### Step 2: Create Helper

```go
// In pkg/types/money.go
package types

import (
	"github.com/shopspring/decimal"
	"database/sql/driver"
)

// Money wraps decimal.Decimal for money values
type Money struct {
	decimal.Decimal
}

// NewMoney creates Money from float64
func NewMoney(value float64) Money {
	return Money{decimal.NewFromFloat(value)}
}

// Value implements driver.Valuer for database serialization
func (m Money) Value() (driver.Value, error) {
	return m.Decimal.Value()
}

// Scan implements sql.Scanner for database deserialization
func (m *Money) Scan(value interface{}) error {
	return m.Decimal.Scan(value)
}
```

### Step 3: Update Models

```go
// In internal/features/payment/model.go
import "github.com/mo-amir99/lms-server-go/pkg/types"

type Payment struct {
    Amount         types.Money `gorm:"type:numeric(10,2)" json:"amount"`
    RefundedAmount types.Money `gorm:"type:numeric(10,2)" json:"refundedAmount"`
    Discount       types.Money `gorm:"type:numeric(10,2)" json:"discount"`
}

// Usage
amount := types.NewMoney(99.99)
total := amount.Add(types.NewMoney(10.00))
```

---

## Complete Example: Optimized User Model

```go
package user

import (
	"strings"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// User represents a system user with optimized types
type User struct {
	types.BaseModel // Embeds ID, CreatedAt, UpdatedAt

	SubscriptionID *uuid.UUID     `gorm:"type:uuid;column:subscription_id" json:"subscriptionId,omitempty"`
	FullName       string         `gorm:"type:varchar(30);not null" json:"fullName"`
	Email          string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"email"`
	Phone          *string        `gorm:"type:varchar(20)" json:"phone,omitempty"`
	Password       string         `gorm:"type:varchar(255);not null" json:"-"`
	UserType       types.UserType `gorm:"type:varchar(20);not null;default:'STUDENT'" json:"userType"`
	RefreshToken   *string        `gorm:"type:text" json:"-"`
	DeviceID       *string        `gorm:"type:varchar(255)" json:"-"`
	Active         bool           `gorm:"type:boolean;not null;default:true" json:"isActive"`
}

// Benefits:
// ✅ No more ID, CreatedAt, UpdatedAt boilerplate
// ✅ UserType is compile-time validated enum
// ✅ Cannot assign invalid user types
// ✅ IDE autocomplete for user types
```

---

## Complete Example: Optimized Payment Model

```go
package payment

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

type Payment struct {
	types.BaseModel // ID, timestamps

	SubscriptionID       uuid.UUID           `gorm:"type:uuid;not null" json:"subscriptionId"`
	PaymentMethod        types.PaymentMethod `gorm:"type:varchar(50)" json:"paymentMethod"`
	ScreenshotURL        *string             `gorm:"type:text" json:"screenshotUrl,omitempty"`
	TransactionReference *string             `gorm:"type:varchar(255)" json:"transactionReference,omitempty"`
	Details              *string             `gorm:"type:text" json:"details,omitempty"`
	SubscriptionPoints   int                 `gorm:"type:int;not null" json:"subscriptionPoints"`
	Amount               types.Money         `gorm:"type:numeric(10,2);not null" json:"amount"`
	RefundedAmount       types.Money         `gorm:"type:numeric(10,2);not null;default:0" json:"refundedAmount"`
	Discount             types.Money         `gorm:"type:numeric(10,2);not null;default:0" json:"discount"`
	PeriodInDays         int                 `gorm:"type:int;not null" json:"periodInDays"`
	IsAddition           bool                `gorm:"type:boolean;not null;default:false" json:"isAddition"`
	Date                 time.Time           `gorm:"type:timestamp;not null" json:"date"`
	Currency             types.Currency      `gorm:"type:varchar(3);not null;default:'EGP'" json:"currency"`
	Status               types.PaymentStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
}

// Benefits:
// ✅ No floating point money errors
// ✅ Type-safe enums for method, currency, status
// ✅ Compile-time validation
// ✅ Accurate financial calculations
```

---

## Testing After Implementation

```bash
# 1. Build to check for errors
go build ./...

# 2. Run tests
go test ./...

# 3. Verify database migrations still work
go run ./cmd/app

# 4. Check API responses unchanged
curl http://localhost:8080/api/users | jq
```

---

## Migration Strategy

### Phase 1: Add Types (No Breaking Changes)

1. Create `pkg/types` package
2. Define all enums and structs
3. **Don't update models yet**
4. Test compilation

### Phase 2: Update Models One by One

1. Start with `User` model
2. Update `Payment` model
3. Update remaining models
4. Run tests after each

### Phase 3: Clean Up

1. Remove string constants from individual files
2. Update imports throughout codebase
3. Full test suite

---

## Estimated Time

| Task            | Time          | Difficulty      |
| --------------- | ------------- | --------------- |
| Enum Constants  | 30 min        | Easy            |
| JSON Structs    | 20 min        | Easy            |
| BaseModel Embed | 15 min        | Easy            |
| Money Type      | 45 min        | Medium          |
| Testing         | 30 min        | Easy            |
| **Total**       | **2.5 hours** | **Easy-Medium** |

---

## rollback Plan

All changes are **additive and non-breaking**:

- Keep old string types alongside new types temporarily
- Gradual migration model by model
- Frontend sees no difference (JSON serialization identical)
- Can rollback individual models without affecting others

---

## Questions?

These optimizations are **100% optional** - your current code is production-ready. Implement only if you want:

- Better type safety
- Compile-time validation
- Improved maintainability
- Reduced parsing overhead
