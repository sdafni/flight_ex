package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"flight-booking-system/internal/models"
)

// ReserveSeats reserves seats for an order with row-level locking
func (db *DB) ReserveSeats(flightID string, seats []string, orderID, userID string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock rows for update
	placeholders := strings.Repeat("?,", len(seats))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
		SELECT seat_id, seat_number, status, reserved_at
		FROM seats
		WHERE flight_id = ? AND seat_number IN (%s)
		FOR UPDATE
	`, placeholders)

	args := make([]interface{}, 0, len(seats)+1)
	args = append(args, flightID)
	for _, seat := range seats {
		args = append(args, seat)
	}

	rows, err := tx.Query(query, args...)
	if err != nil {
		return fmt.Errorf("failed to lock seats: %w", err)
	}
	defer rows.Close()

	// Check availability (including expired reservations)
	foundSeats := make(map[string]bool)
	for rows.Next() {
		var seatID, seatNumber, status string
		var reservedAt sql.NullTime

		if err := rows.Scan(&seatID, &seatNumber, &status, &reservedAt); err != nil {
			return fmt.Errorf("failed to scan seat: %w", err)
		}

		foundSeats[seatNumber] = true

		// Check if seat is available or reservation expired
		if status == models.SeatAvailable {
			continue
		} else if status == models.SeatReserved && reservedAt.Valid {
			if time.Since(reservedAt.Time) > 15*time.Minute {
				continue // Expired reservation, can be taken
			}
		}

		return fmt.Errorf("seat %s: %w", seatNumber, ErrSeatNotAvailable)
	}

	// Verify all requested seats exist
	for _, seat := range seats {
		if !foundSeats[seat] {
			return fmt.Errorf("seat %s: %w", seat, ErrSeatNotExist)
		}
	}

	// Reserve the seats
	updateQuery := fmt.Sprintf(`
		UPDATE seats
		SET status = ?, reserved_by = ?, user_id = ?, reserved_at = NOW()
		WHERE flight_id = ? AND seat_number IN (%s)
	`, placeholders)

	updateArgs := make([]interface{}, 0, len(seats)+4)
	updateArgs = append(updateArgs, models.SeatReserved, orderID, userID, flightID)
	for _, seat := range seats {
		updateArgs = append(updateArgs, seat)
	}

	if _, err := tx.Exec(updateQuery, updateArgs...); err != nil {
		return fmt.Errorf("failed to reserve seats: %w", err)
	}

	return tx.Commit()
}

// ReleaseSeats releases seats reserved by an order
func (db *DB) ReleaseSeats(orderID string) error {
	query := `
		UPDATE seats
		SET status = ?, reserved_by = NULL, user_id = NULL, reserved_at = NULL
		WHERE reserved_by = ?
	`

	_, err := db.Exec(query, models.SeatAvailable, orderID)
	if err != nil {
		return fmt.Errorf("failed to release seats: %w", err)
	}

	return nil
}

// UpdateSeats updates seat selection for an order
func (db *DB) UpdateSeats(orderID string, oldSeats, newSeats []string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get flight ID and user ID from the order
	var flightID, userID string
	err = tx.QueryRow("SELECT flight_id, user_id FROM orders WHERE order_id = ?", orderID).Scan(&flightID, &userID)
	if err != nil {
		return fmt.Errorf("failed to get order info: %w", err)
	}

	// Release old seats
	if len(oldSeats) > 0 {
		placeholders := strings.Repeat("?,", len(oldSeats))
		placeholders = placeholders[:len(placeholders)-1]

		releaseQuery := fmt.Sprintf(`
			UPDATE seats
			SET status = ?, reserved_by = NULL, user_id = NULL, reserved_at = NULL
			WHERE reserved_by = ? AND seat_number IN (%s)
		`, placeholders)

		args := make([]interface{}, 0, len(oldSeats)+2)
		args = append(args, models.SeatAvailable, orderID)
		for _, seat := range oldSeats {
			args = append(args, seat)
		}

		if _, err := tx.Exec(releaseQuery, args...); err != nil {
			return fmt.Errorf("failed to release old seats: %w", err)
		}
	}

	// Reserve new seats (with locking)
	if len(newSeats) > 0 {
		placeholders := strings.Repeat("?,", len(newSeats))
		placeholders = placeholders[:len(placeholders)-1]

		lockQuery := fmt.Sprintf(`
			SELECT seat_id, seat_number, status, reserved_at
			FROM seats
			WHERE flight_id = ? AND seat_number IN (%s)
			FOR UPDATE
		`, placeholders)

		args := make([]interface{}, 0, len(newSeats)+1)
		args = append(args, flightID)
		for _, seat := range newSeats {
			args = append(args, seat)
		}

		rows, err := tx.Query(lockQuery, args...)
		if err != nil {
			return fmt.Errorf("failed to lock new seats: %w", err)
		}
		defer rows.Close()

		foundSeats := make(map[string]bool)
		for rows.Next() {
			var seatID, seatNumber, status string
			var reservedAt sql.NullTime

			if err := rows.Scan(&seatID, &seatNumber, &status, &reservedAt); err != nil {
				return fmt.Errorf("failed to scan seat: %w", err)
			}

			foundSeats[seatNumber] = true

			if status == models.SeatAvailable {
				continue
			} else if status == models.SeatReserved && reservedAt.Valid {
				if time.Since(reservedAt.Time) > 15*time.Minute {
					continue
				}
			}

			return fmt.Errorf("seat %s is not available", seatNumber)
		}

		for _, seat := range newSeats {
			if !foundSeats[seat] {
				return fmt.Errorf("seat %s does not exist", seat)
			}
		}

		// Reserve new seats
		reserveQuery := fmt.Sprintf(`
			UPDATE seats
			SET status = ?, reserved_by = ?, user_id = ?, reserved_at = NOW()
			WHERE flight_id = ? AND seat_number IN (%s)
		`, placeholders)

		reserveArgs := make([]interface{}, 0, len(newSeats)+4)
		reserveArgs = append(reserveArgs, models.SeatReserved, orderID, userID, flightID)
		for _, seat := range newSeats {
			reserveArgs = append(reserveArgs, seat)
		}

		if _, err := tx.Exec(reserveQuery, reserveArgs...); err != nil {
			return fmt.Errorf("failed to reserve new seats: %w", err)
		}
	}

	return tx.Commit()
}

// ConfirmSeats confirms seats for an order (mark as BOOKED)
func (db *DB) ConfirmSeats(orderID string) error {
	query := `
		UPDATE seats
		SET status = ?
		WHERE reserved_by = ?
	`

	_, err := db.Exec(query, models.SeatBooked, orderID)
	if err != nil {
		return fmt.Errorf("failed to confirm seats: %w", err)
	}

	return nil
}

// GetSeats retrieves all seats for a flight
func (db *DB) GetSeats(flightID string) ([]models.Seat, error) {
	query := `
		SELECT seat_id, flight_id, seat_number, status, reserved_by, user_id, reserved_at
		FROM seats
		WHERE flight_id = ?
		ORDER BY seat_number
	`

	rows, err := db.Query(query, flightID)
	if err != nil {
		return nil, fmt.Errorf("failed to query seats: %w", err)
	}
	defer rows.Close()

	var seats []models.Seat
	for rows.Next() {
		var seat models.Seat
		var reservedBy, userID sql.NullString
		var reservedAt sql.NullTime

		err := rows.Scan(&seat.SeatID, &seat.FlightID, &seat.SeatNumber, &seat.Status,
			&reservedBy, &userID, &reservedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan seat: %w", err)
		}

		if reservedBy.Valid {
			seat.ReservedBy = &reservedBy.String
		}
		if userID.Valid {
			seat.UserID = &userID.String
		}
		if reservedAt.Valid {
			seat.ReservedAt = &reservedAt.Time
		}

		seats = append(seats, seat)
	}

	return seats, nil
}

// CreateOrder creates a new order
func (db *DB) CreateOrder(order *models.Order) error {
	query := `
		INSERT INTO orders (order_id, flight_id, user_id, status, workflow_id, run_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query, order.OrderID, order.FlightID, order.UserID,
		order.Status, order.WorkflowID, order.RunID)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	return nil
}

// GetOrder retrieves an order by ID
func (db *DB) GetOrder(orderID string) (*models.Order, error) {
	query := `
		SELECT order_id, flight_id, user_id, status, workflow_id, run_id, created_at, updated_at
		FROM orders
		WHERE order_id = ?
	`

	var order models.Order
	err := db.QueryRow(query, orderID).Scan(
		&order.OrderID, &order.FlightID, &order.UserID, &order.Status,
		&order.WorkflowID, &order.RunID, &order.CreatedAt, &order.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("order not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &order, nil
}

// UpdateOrderStatus updates an order's status
func (db *DB) UpdateOrderStatus(orderID, status string) error {
	query := `
		UPDATE orders
		SET status = ?, updated_at = NOW()
		WHERE order_id = ?
	`

	result, err := db.Exec(query, status, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return errors.New("order not found")
	}

	return nil
}

// GetOrderSeats retrieves seats reserved for an order
func (db *DB) GetOrderSeats(orderID string) ([]string, error) {
	query := `
		SELECT seat_number
		FROM seats
		WHERE reserved_by = ?
		ORDER BY seat_number
	`

	rows, err := db.Query(query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query order seats: %w", err)
	}
	defer rows.Close()

	var seats []string
	for rows.Next() {
		var seatNumber string
		if err := rows.Scan(&seatNumber); err != nil {
			return nil, fmt.Errorf("failed to scan seat number: %w", err)
		}
		seats = append(seats, seatNumber)
	}

	return seats, nil
}

// CreatePayment creates a payment record
func (db *DB) CreatePayment(payment *models.Payment) error {
	query := `
		INSERT INTO payments (payment_id, order_id, payment_code, transaction_id, status, attempts)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query, payment.PaymentID, payment.OrderID, payment.PaymentCode,
		payment.TransactionID, payment.Status, payment.Attempts)
	if err != nil {
		return fmt.Errorf("failed to create payment: %w", err)
	}

	return nil
}

// UpdatePayment updates a payment record
func (db *DB) UpdatePayment(paymentID, status string, attempts int, transactionID *string) error {
	query := `
		UPDATE payments
		SET status = ?, attempts = ?, transaction_id = ?, updated_at = NOW()
		WHERE payment_id = ?
	`

	_, err := db.Exec(query, status, attempts, transactionID, paymentID)
	if err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	return nil
}

// ResetFlightSeats resets all seats for a flight (for testing/admin)
func (db *DB) ResetFlightSeats(flightID string) error {
	query := `
		UPDATE seats
		SET status = ?, reserved_by = NULL, user_id = NULL, reserved_at = NULL
		WHERE flight_id = ?
	`

	_, err := db.Exec(query, models.SeatAvailable, flightID)
	if err != nil {
		return fmt.Errorf("failed to reset seats: %w", err)
	}

	return nil
}

// DeleteOrdersByFlight deletes all orders for a flight (for testing/admin)
func (db *DB) DeleteOrdersByFlight(flightID string) error {
	query := `DELETE FROM orders WHERE flight_id = ?`

	_, err := db.Exec(query, flightID)
	if err != nil {
		return fmt.Errorf("failed to delete orders: %w", err)
	}

	return nil
}
