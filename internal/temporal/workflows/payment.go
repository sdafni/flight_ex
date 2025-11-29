package workflows

import (
	"fmt"
	"time"

	"flight-booking-system/internal/models"
	"flight-booking-system/internal/temporal/activities"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// PaymentValidationWorkflow validates payment with retries
func PaymentValidationWorkflow(ctx workflow.Context, paymentCode string, orderID string) (*models.PaymentResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("PaymentValidationWorkflow started", "orderID", orderID)

	// Activity options with 10-second timeout
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var paymentActivities *activities.PaymentActivities
	var result *models.PaymentResult

	// Try to validate payment (with automatic retries)
	err := workflow.ExecuteActivity(ctx, paymentActivities.ValidatePayment, paymentCode, orderID).Get(ctx, &result)
	if err != nil {
		logger.Error("Payment validation failed after retries", "error", err)
		return &models.PaymentResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("payment validation failed: %v", err),
		}, err
	}

	if !result.Success {
		logger.Error("Payment validation unsuccessful", "errorMessage", result.ErrorMessage)
		return result, fmt.Errorf("payment validation failed: %s", result.ErrorMessage)
	}

	logger.Info("Payment validation successful", "transactionID", result.TransactionID)
	return result, nil
}
