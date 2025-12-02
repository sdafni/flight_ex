package models

import "time"

// Order statuses
const (
	StatusCreated        = "CREATED"
	StatusSeatsReserved  = "SEATS_RESERVED"
	StatusPaymentPending = "PAYMENT_PENDING"
	StatusConfirmed      = "CONFIRMED"
	StatusFailed         = "FAILED"
	StatusExpired        = "EXPIRED"
	StatusCancelled      = "CANCELLED"
)

// Seat statuses
const (
	SeatAvailable = "AVAILABLE"
	SeatReserved  = "RESERVED"
	SeatBooked    = "BOOKED"
)

// Order represents a flight booking order
type Order struct {
	OrderID    string    `json:"orderId" db:"order_id"`
	FlightID   string    `json:"flightId" db:"flight_id"`
	UserID     string    `json:"userId" db:"user_id"`
	Status     string    `json:"status" db:"status"`
	WorkflowID string    `json:"workflowId" db:"workflow_id"`
	RunID      string    `json:"runId" db:"run_id"`
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt  time.Time `json:"updatedAt" db:"updated_at"`
}

// Seat represents a flight seat
type Seat struct {
	SeatID     string     `json:"seatId" db:"seat_id"`
	FlightID   string     `json:"flightId" db:"flight_id"`
	SeatNumber string     `json:"seatNumber" db:"seat_number"`
	Status     string     `json:"status" db:"status"`
	ReservedBy *string    `json:"reservedBy,omitempty" db:"reserved_by"`
	UserID     *string    `json:"userId,omitempty" db:"user_id"`
	ReservedAt *time.Time `json:"reservedAt,omitempty" db:"reserved_at"`
}

// Payment represents a payment transaction
type Payment struct {
	PaymentID     string    `json:"paymentId" db:"payment_id"`
	OrderID       string    `json:"orderId" db:"order_id"`
	PaymentCode   string    `json:"paymentCode" db:"payment_code"`
	TransactionID *string   `json:"transactionId,omitempty" db:"transaction_id"`
	Status        string    `json:"status" db:"status"`
	Attempts      int       `json:"attempts" db:"attempts"`
	CreatedAt     time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time `json:"updatedAt" db:"updated_at"`
}

// BookingInput represents workflow input
type BookingInput struct {
	OrderID  string   `json:"orderId"`
	FlightID string   `json:"flightId"`
	UserID   string   `json:"userId"`
	Seats    []string `json:"seats"`
}

// BookingState represents the current workflow state
type BookingState struct {
	OrderID            string    `json:"orderId"`
	FlightID           string    `json:"flightId"`
	UserID             string    `json:"userId"`
	Seats              []string  `json:"seats"`
	Status             string    `json:"status"`
	ReservationStartAt time.Time `json:"reservationStartAt"`
	PaymentAttempts    int       `json:"paymentAttempts"`
	// Note: TimeRemaining is NOT stored here because workflow.Now() is deterministic
	// and doesn't advance during idle periods. Calculate it server-side using wall-clock time.
}

// BookingResult represents workflow output
type BookingResult struct {
	State *BookingState `json:"state"`
}

// PaymentResult represents payment validation result
type PaymentResult struct {
	Success       bool   `json:"success"`
	TransactionID string `json:"transactionId,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
}

// API Request/Response models

type CreateOrderRequest struct {
	UserID string   `json:"userId"`
	Seats  []string `json:"seats"`
}

type CreateOrderResponse struct {
	OrderID    string   `json:"orderId"`
	FlightID   string   `json:"flightId"`
	UserID     string   `json:"userId"`
	Seats      []string `json:"seats"`
	Status     string   `json:"status"`
	WorkflowID string   `json:"workflowId"`
}

type UpdateSeatsRequest struct {
	Seats []string `json:"seats"`
}

type SubmitPaymentRequest struct {
	PaymentCode string `json:"paymentCode"`
}

type OrderStatusResponse struct {
	OrderID       string   `json:"orderId"`
	FlightID      string   `json:"flightId"`
	UserID        string   `json:"userId"`
	Seats         []string `json:"seats"`
	Status        string   `json:"status"`
	TimeRemaining int64    `json:"timeRemaining"` // seconds
	ReservedAt    *time.Time `json:"reservedAt,omitempty"`
}

type SeatsResponse struct {
	FlightID string `json:"flightId"`
	Seats    []Seat `json:"seats"`
}
