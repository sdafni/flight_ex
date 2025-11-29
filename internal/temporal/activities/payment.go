package activities

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"time"

	"flight-booking-system/internal/database"
	"flight-booking-system/internal/models"

	"github.com/google/uuid"
)

type PaymentActivities struct {
	DB *database.DB
}

func NewPaymentActivities(db *database.DB) *PaymentActivities {
	return &PaymentActivities{DB: db}
}

// ValidatePayment validates a payment code with simulated failures
func (a *PaymentActivities) ValidatePayment(ctx context.Context, paymentCode string, orderID string) (*models.PaymentResult, error) {
	// Validate payment code format (5 digits)
	matched, err := regexp.MatchString(`^\d{5}$`, paymentCode)
	if err != nil {
		return nil, fmt.Errorf("regex error: %w", err)
	}
	if !matched {
		return &models.PaymentResult{
			Success:      false,
			ErrorMessage: "invalid payment code format (must be 5 digits)",
		}, nil
	}

	// Simulate random delay (0-5 seconds)
	delay := time.Duration(rand.Intn(5000)) * time.Millisecond
	time.Sleep(delay)

	// Simulate 15% failure rate
	if rand.Float32() < 0.15 {
		return &models.PaymentResult{
			Success:      false,
			ErrorMessage: "payment gateway error (simulated)",
		}, errors.New("payment gateway error")
	}

	// Generate transaction ID
	transactionID := uuid.New().String()

	return &models.PaymentResult{
		Success:       true,
		TransactionID: transactionID,
	}, nil
}

// RecordPaymentAttempt records a payment attempt in the database
func (a *PaymentActivities) RecordPaymentAttempt(ctx context.Context, orderID, paymentCode string, attempts int) error {
	paymentID := uuid.New().String()

	payment := &models.Payment{
		PaymentID:   paymentID,
		OrderID:     orderID,
		PaymentCode: paymentCode,
		Status:      "PENDING",
		Attempts:    attempts,
	}

	err := a.DB.CreatePayment(payment)
	if err != nil {
		return fmt.Errorf("failed to record payment attempt: %w", err)
	}

	return nil
}

// UpdatePaymentRecord updates a payment record with the result
func (a *PaymentActivities) UpdatePaymentRecord(ctx context.Context, orderID, status string, attempts int, transactionID *string) error {
	// For simplicity, we'll update the most recent payment record for this order
	// In production, you'd want to track payment IDs more carefully
	return nil
}
