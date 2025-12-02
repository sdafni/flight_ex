package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"flight-booking-system/internal/config"
	"flight-booking-system/internal/database"
	"flight-booking-system/internal/models"
	"flight-booking-system/internal/temporal/workflows"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.temporal.io/sdk/client"
)

type Handler struct {
	DB             *database.DB
	TemporalClient client.Client
}

func NewHandler(db *database.DB, temporalClient client.Client) *Handler {
	return &Handler{
		DB:             db,
		TemporalClient: temporalClient,
	}
}

// Health check endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// CreateOrder creates a new booking order
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flightID := vars["flightId"]

	var req models.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate
	if req.UserID == "" || len(req.Seats) == 0 {
		http.Error(w, "userId and seats required", http.StatusBadRequest)
		return
	}

	orderID := uuid.New().String()

	// Start Temporal workflow
	workflowOptions := client.StartWorkflowOptions{
		ID:        orderID,
		TaskQueue: "booking-task-queue",
	}

	input := models.BookingInput{
		OrderID:  orderID,
		FlightID: flightID,
		UserID:   req.UserID,
		Seats:    req.Seats,
	}

	we, err := h.TemporalClient.ExecuteWorkflow(context.Background(), workflowOptions,
		workflows.BookingWorkflow, input)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to start workflow: %v", err), http.StatusInternalServerError)
		return
	}

	// Store order in database
	order := &models.Order{
		OrderID:    orderID,
		FlightID:   flightID,
		UserID:     req.UserID,
		Status:     models.StatusCreated,
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
	}
	//yuvald TODO why not orm?
	if err := h.DB.CreateOrder(order); err != nil {
		http.Error(w, fmt.Sprintf("failed to create order: %v", err), http.StatusInternalServerError)
		return
	}

	// Wait a moment for workflow to process seat reservation
	time.Sleep(100 * time.Millisecond)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.CreateOrderResponse{
		OrderID:    orderID,
		FlightID:   flightID,
		UserID:     req.UserID,
		Seats:      req.Seats,
		Status:     models.StatusCreated,
		WorkflowID: we.GetID(),
	})
}

// GetOrderStatus retrieves the status of an order
func (h *Handler) GetOrderStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	// Get order from database
	order, err := h.DB.GetOrder(orderID)
	if err != nil {
		http.Error(w, fmt.Sprintf("order not found: %v", err), http.StatusNotFound)
		return
	}

	// Query workflow for current state
	resp, err := h.TemporalClient.QueryWorkflow(context.Background(), order.WorkflowID, order.RunID, workflows.QueryGetStatus)
	if err != nil {
		// If workflow is not running, use database status
		seats, _ := h.DB.GetOrderSeats(orderID)
		response := models.OrderStatusResponse{
			OrderID:       order.OrderID,
			FlightID:      order.FlightID,
			UserID:        order.UserID,
			Seats:         seats,
			Status:        order.Status,
			TimeRemaining: 0,
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	var state *models.BookingState
	if err := resp.Get(&state); err != nil {
		http.Error(w, fmt.Sprintf("failed to get workflow state: %v", err), http.StatusInternalServerError)
		return
	}

	// Calculate time remaining on the server side using actual current time
	// (workflow.Now() in query handlers returns deterministic time that doesn't advance)
	cfg := config.Load()
	elapsed := time.Since(state.ReservationStartAt)
	remaining := cfg.ReservationTimeout - elapsed
	if remaining < 0 {
		remaining = 0
	}
	timeRemaining := int64(remaining.Seconds())

	response := models.OrderStatusResponse{
		OrderID:       state.OrderID,
		FlightID:      state.FlightID,
		UserID:        state.UserID,
		Seats:         state.Seats,
		Status:        state.Status,
		TimeRemaining: timeRemaining,
		ReservedAt:    &state.ReservationStartAt,
	}

	json.NewEncoder(w).Encode(response)
}

// UpdateSeats updates seat selection for an order
func (h *Handler) UpdateSeats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	var req models.UpdateSeatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate
	if len(req.Seats) == 0 {
		http.Error(w, "seats required", http.StatusBadRequest)
		return
	}

	// Get order from database
	order, err := h.DB.GetOrder(orderID)
	if err != nil {
		http.Error(w, fmt.Sprintf("order not found: %v", err), http.StatusNotFound)
		return
	}

	// Send signal to workflow
	err = h.TemporalClient.SignalWorkflow(context.Background(), order.WorkflowID, order.RunID,
		workflows.SignalUpdateSeats, req.Seats)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to send signal: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "seats updated"})
}

// SubmitPayment submits payment for an order
func (h *Handler) SubmitPayment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	var req models.SubmitPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate payment code format
	if len(req.PaymentCode) != 5 {
		http.Error(w, "payment code must be 5 digits", http.StatusBadRequest)
		return
	}

	// Get order from database
	order, err := h.DB.GetOrder(orderID)
	if err != nil {
		http.Error(w, fmt.Sprintf("order not found: %v", err), http.StatusNotFound)
		return
	}

	// Send signal to workflow
	err = h.TemporalClient.SignalWorkflow(context.Background(), order.WorkflowID, order.RunID,
		workflows.SignalSubmitPayment, req.PaymentCode)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to send signal: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "payment submitted"})
}

// CancelOrder cancels an order
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	// Get order from database
	order, err := h.DB.GetOrder(orderID)
	if err != nil {
		http.Error(w, fmt.Sprintf("order not found: %v", err), http.StatusNotFound)
		return
	}

	// Send signal to workflow
	err = h.TemporalClient.SignalWorkflow(context.Background(), order.WorkflowID, order.RunID,
		workflows.SignalCancelOrder, true)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to send signal: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "order cancelled"})
}

// GetSeats retrieves available seats for a flight
func (h *Handler) GetSeats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flightID := vars["flightId"]

	seats, err := h.DB.GetSeats(flightID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get seats: %v", err), http.StatusInternalServerError)
		return
	}

	response := models.SeatsResponse{
		FlightID: flightID,
		Seats:    seats,
	}

	json.NewEncoder(w).Encode(response)
}

// ResetFlight resets all seats for a flight (admin/testing)
func (h *Handler) ResetFlight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flightID := vars["flightId"]

	// Delete all orders for the flight
	if err := h.DB.DeleteOrdersByFlight(flightID); err != nil {
		http.Error(w, fmt.Sprintf("failed to delete orders: %v", err), http.StatusInternalServerError)
		return
	}

	// Reset all seats
	if err := h.DB.ResetFlightSeats(flightID); err != nil {
		http.Error(w, fmt.Sprintf("failed to reset seats: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "flight reset"})
}
