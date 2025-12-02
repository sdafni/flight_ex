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

// UpdatePaymentRecord creates or updates a payment record with the result
func (a *PaymentActivities) UpdatePaymentRecord(ctx context.Context, orderID, paymentCode, status string, transactionID *string, errorMessage *string) error {
	// First, try to update existing record
	updateQuery := `
		UPDATE payments
		SET status = ?, transaction_id = ?, error_message = ?, updated_at = NOW()
		WHERE order_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	result, err := a.DB.Exec(updateQuery, status, transactionID, errorMessage, orderID)
	if err != nil {
		return fmt.Errorf("failed to update payment record: %w", err)
	}

	// If no rows were updated, create a new record
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Create new payment record
		paymentID := uuid.New().String()
		payment := &models.Payment{
			PaymentID:     paymentID,
			OrderID:       orderID,
			PaymentCode:   paymentCode,
			Status:        status,
			TransactionID: transactionID,
			ErrorMessage:  errorMessage,
		}

		err := a.DB.CreatePayment(payment)
		if err != nil {
			return fmt.Errorf("failed to create payment record: %w", err)
		}
	}

	return nil
}
