package activities

import (
	"context"
	"errors"
	"fmt"
	"log"

	"flight-booking-system/internal/database"
	"go.temporal.io/sdk/temporal"
)

type OrderActivities struct {
	DB *database.DB
}

func NewOrderActivities(db *database.DB) *OrderActivities {
	return &OrderActivities{DB: db}
}

// UpdateOrderStatus updates an order's status
func (a *OrderActivities) UpdateOrderStatus(ctx context.Context, orderID, status string) error {
	err := a.DB.UpdateOrderStatus(orderID, status)
	if err != nil {
		// Order not found is a permanent error - don't retry
		if errors.Is(err, database.ErrOrderNotFound) {
			return temporal.NewNonRetryableApplicationError(
				err.Error(),
				"OrderNotFound",
				err,
			)
		}
		return fmt.Errorf("failed to update order status: %w", err)
	}
	return nil
}

// SendConfirmation sends a booking confirmation (simulated)
func (a *OrderActivities) SendConfirmation(ctx context.Context, orderID string) error {
	// In production, this would send an email/SMS
	log.Printf("Sending confirmation for order %s", orderID)
	return nil
}
