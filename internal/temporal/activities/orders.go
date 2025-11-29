package activities

import (
	"context"
	"fmt"
	"log"

	"flight-booking-system/internal/database"
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
