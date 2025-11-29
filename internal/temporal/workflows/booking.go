package workflows

import (
	"time"

	"flight-booking-system/internal/config"
	"flight-booking-system/internal/models"
	"flight-booking-system/internal/temporal/activities"

	"go.temporal.io/sdk/workflow"
)

const (
	SignalUpdateSeats   = "updateSeats"
	SignalSubmitPayment = "submitPayment"
	SignalCancelOrder   = "cancelOrder"
	QueryGetStatus      = "getStatus"
)

// BookingWorkflow orchestrates the entire booking lifecycle
func BookingWorkflow(ctx workflow.Context, input models.BookingInput) (*models.BookingResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("BookingWorkflow started", "orderID", input.OrderID)

	// Load configuration
	cfg := config.Load()

	// Initialize workflow state
	state := &models.BookingState{
		OrderID:            input.OrderID,
		FlightID:           input.FlightID,
		UserID:             input.UserID,
		Status:             models.StatusCreated,
		PaymentAttempts:    0,
		ReservationStartAt: workflow.Now(ctx),
	}

	// Set up activity options
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	activityCtx := workflow.WithActivityOptions(ctx, activityOptions)

	// Set up query handler for real-time status
	err := workflow.SetQueryHandler(ctx, QueryGetStatus, func() (*models.BookingState, error) {
		// Calculate time remaining
		elapsed := workflow.Now(ctx).Sub(state.ReservationStartAt)
		remaining := cfg.ReservationTimeout - elapsed
		if remaining < 0 {
			remaining = 0
		}
		state.TimeRemaining = int64(remaining.Seconds())
		return state, nil
	})
	if err != nil {
		return nil, err
	}

	// Set up signal channels
	seatUpdateChan := workflow.GetSignalChannel(ctx, SignalUpdateSeats)
	paymentChan := workflow.GetSignalChannel(ctx, SignalSubmitPayment)
	cancelChan := workflow.GetSignalChannel(ctx, SignalCancelOrder)

	// Reserve initial seats
	var seatActivities *activities.SeatActivities
	err = workflow.ExecuteActivity(activityCtx, seatActivities.ReserveSeats,
		input.FlightID, input.Seats, input.OrderID, input.UserID).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to reserve initial seats", "error", err)
		state.Status = models.StatusFailed
		return &models.BookingResult{State: state}, err
	}

	state.Seats = input.Seats
	state.Status = models.StatusSeatsReserved
	state.ReservationStartAt = workflow.Now(ctx)

	// Update order status
	var orderActivities *activities.OrderActivities
	workflow.ExecuteActivity(activityCtx, orderActivities.UpdateOrderStatus,
		input.OrderID, models.StatusSeatsReserved).Get(ctx, nil)

	// Start reservation timer
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timerFuture := workflow.NewTimer(timerCtx, cfg.ReservationTimeout)

	// Main event loop
	for {
		selector := workflow.NewSelector(ctx)

		// Handle seat updates
		selector.AddReceive(seatUpdateChan, func(c workflow.ReceiveChannel, more bool) {
			var newSeats []string
			c.Receive(ctx, &newSeats)

			logger.Info("Received seat update signal", "newSeats", newSeats)

			// Update seats
			err := workflow.ExecuteActivity(activityCtx, seatActivities.UpdateSeats,
				state.OrderID, state.Seats, newSeats).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to update seats", "error", err)
				return
			}

			state.Seats = newSeats
			state.ReservationStartAt = workflow.Now(ctx)

			// Cancel old timer and start new one
			cancelTimer()
			timerCtx, cancelTimer = workflow.WithCancel(ctx)
			timerFuture = workflow.NewTimer(timerCtx, cfg.ReservationTimeout)

			logger.Info("Seat update complete, timer reset", "newSeats", newSeats)
		})

		// Handle payment submission
		selector.AddReceive(paymentChan, func(c workflow.ReceiveChannel, more bool) {
			var paymentCode string
			c.Receive(ctx, &paymentCode)

			logger.Info("Received payment signal", "paymentCode", paymentCode)

			state.Status = models.StatusPaymentPending
			workflow.ExecuteActivity(activityCtx, orderActivities.UpdateOrderStatus,
				state.OrderID, models.StatusPaymentPending).Get(ctx, nil)

			// Execute payment validation child workflow
			childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID: state.OrderID + "-payment",
			})

			var paymentResult *models.PaymentResult
			err := workflow.ExecuteChildWorkflow(childCtx, PaymentValidationWorkflow,
				paymentCode, state.OrderID).Get(ctx, &paymentResult)

			if err == nil && paymentResult.Success {
				logger.Info("Payment successful", "transactionID", paymentResult.TransactionID)

				state.Status = models.StatusConfirmed

				// Confirm seats (mark as BOOKED)
				workflow.ExecuteActivity(activityCtx, seatActivities.ConfirmSeats, state.OrderID).Get(ctx, nil)

				// Update order status
				workflow.ExecuteActivity(activityCtx, orderActivities.UpdateOrderStatus,
					state.OrderID, models.StatusConfirmed).Get(ctx, nil)

				// Send confirmation
				workflow.ExecuteActivity(activityCtx, orderActivities.SendConfirmation,
					state.OrderID).Get(ctx, nil)

				logger.Info("Booking confirmed", "orderID", state.OrderID)
			} else {
				logger.Error("Payment failed", "error", err)

				state.Status = models.StatusFailed

				// Release seats
				workflow.ExecuteActivity(activityCtx, seatActivities.ReleaseSeats, state.OrderID).Get(ctx, nil)

				// Update order status
				workflow.ExecuteActivity(activityCtx, orderActivities.UpdateOrderStatus,
					state.OrderID, models.StatusFailed).Get(ctx, nil)

				logger.Info("Order failed due to payment failure", "orderID", state.OrderID)
			}
		})

		// Handle cancellation
		selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, more bool) {
			var cancel bool
			c.Receive(ctx, &cancel)

			logger.Info("Received cancel signal", "orderID", state.OrderID)

			state.Status = models.StatusCancelled

			// Release seats
			workflow.ExecuteActivity(activityCtx, seatActivities.ReleaseSeats, state.OrderID).Get(ctx, nil)

			// Update order status
			workflow.ExecuteActivity(activityCtx, orderActivities.UpdateOrderStatus,
				state.OrderID, models.StatusCancelled).Get(ctx, nil)

			logger.Info("Order cancelled", "orderID", state.OrderID)
		})

		// Handle timer expiration
		selector.AddFuture(timerFuture, func(f workflow.Future) {
			err := f.Get(ctx, nil)
			if err != nil {
				// Timer was cancelled (likely due to seat update)
				logger.Info("Timer cancelled")
				return
			}

			logger.Info("Reservation timer expired", "orderID", state.OrderID)

			state.Status = models.StatusExpired

			// Release seats
			workflow.ExecuteActivity(activityCtx, seatActivities.ReleaseSeats, state.OrderID).Get(ctx, nil)

			// Update order status
			workflow.ExecuteActivity(activityCtx, orderActivities.UpdateOrderStatus,
				state.OrderID, models.StatusExpired).Get(ctx, nil)

			logger.Info("Order expired", "orderID", state.OrderID)
		})

		selector.Select(ctx)

		// Exit conditions
		if state.Status == models.StatusConfirmed ||
			state.Status == models.StatusFailed ||
			state.Status == models.StatusExpired ||
			state.Status == models.StatusCancelled {
			break
		}
	}

	logger.Info("BookingWorkflow completed", "orderID", input.OrderID, "status", state.Status)
	return &models.BookingResult{State: state}, nil
}
