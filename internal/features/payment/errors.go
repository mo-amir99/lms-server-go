package payment

import (
	"errors"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

var (
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrInvalidStatus        = errors.New("invalid payment status")
	ErrInvalidPaymentMethod = errors.New("invalid payment method")
)

// Re-export PaymentStatus constants from types for backward compatibility
const (
	StatusPending   = types.PaymentStatusPending
	StatusCompleted = types.PaymentStatusCompleted
	StatusFailed    = types.PaymentStatusFailed
	StatusRefunded  = types.PaymentStatusRefunded
)

// Re-export PaymentMethod constants from types for backward compatibility
const (
	MethodCash         = types.PaymentMethodCash
	MethodBankTransfer = types.PaymentMethodBankTransfer
	MethodCreditCard   = types.PaymentMethodCreditCard
	MethodPayPal       = types.PaymentMethodPayPal
	MethodOther        = types.PaymentMethodOther
)

// ValidStatuses returns all valid payment statuses.
func ValidStatuses() []types.PaymentStatus {
	return []types.PaymentStatus{StatusPending, StatusCompleted, StatusFailed, StatusRefunded}
}

// ValidPaymentMethods returns all valid payment methods.
func ValidPaymentMethods() []types.PaymentMethod {
	return []types.PaymentMethod{MethodCash, MethodBankTransfer, MethodCreditCard, MethodPayPal, MethodOther}
}
