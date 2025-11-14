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
	MethodInstapay     = types.PaymentMethodInstapay
	MethodCreditCard   = types.PaymentMethodCreditCard
	MethodCrypto       = types.PaymentMethodCrypto
	MethodMobileWallet = types.PaymentMethodMobileWallet
	MethodBankTransfer = types.PaymentMethodBankTransfer
	MethodGooglePlay   = types.PaymentMethodGooglePlay
	MethodAppStore     = types.PaymentMethodAppStore
	MethodPayPal       = types.PaymentMethodPayPal
	MethodPayoneer     = types.PaymentMethodPayoneer
	MethodStripe       = types.PaymentMethodStripe
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
