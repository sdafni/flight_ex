package activities

import (
	"context"
	"fmt"

	"flight-booking-system/internal/database"
)

type SeatActivities struct {
	DB *database.DB
}

func NewSeatActivities(db *database.DB) *SeatActivities {
	return &SeatActivities{DB: db}
}

// ReserveSeats reserves seats for an order
func (a *SeatActivities) ReserveSeats(ctx context.Context, flightID string, seats []string, orderID, userID string) error {
	err := a.DB.ReserveSeats(flightID, seats, orderID, userID)
	if err != nil {
		return fmt.Errorf("failed to reserve seats: %w", err)
	}
	return nil
}

// ReleaseSeats releases seats reserved by an order
func (a *SeatActivities) ReleaseSeats(ctx context.Context, orderID string) error {
	err := a.DB.ReleaseSeats(orderID)
	if err != nil {
		return fmt.Errorf("failed to release seats: %w", err)
	}
	return nil
}

// UpdateSeats updates seat selection for an order
func (a *SeatActivities) UpdateSeats(ctx context.Context, orderID string, oldSeats, newSeats []string) error {
	err := a.DB.UpdateSeats(orderID, oldSeats, newSeats)
	if err != nil {
		return fmt.Errorf("failed to update seats: %w", err)
	}
	return nil
}

// ConfirmSeats confirms seats for an order (mark as BOOKED)
func (a *SeatActivities) ConfirmSeats(ctx context.Context, orderID string) error {
	err := a.DB.ConfirmSeats(orderID)
	if err != nil {
		return fmt.Errorf("failed to confirm seats: %w", err)
	}
	return nil
}
