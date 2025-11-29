# Flight Booking System with Temporal - Project Prompt

## Overview
Create a flight booking system in Go that uses Temporal for workflow orchestration. The system manages seat reservations with timeouts, payment validation with retries, and handles concurrent user bookings.

## Core Requirements

### 1. Technology Stack
- **Backend**: Go with RESTful API
- **Workflow Engine**: Temporal (running locally via Docker)
- **Database**: MySQL (file-based, local)
- **Frontend**: Simple HTML/JavaScript UI using HTMX for interactivity
- **Testing**: Python test suite using pytest and requests library

### 2. Architecture Components

#### RESTful API Server (Go)
Create the following endpoints:

```
POST   /api/flights/{flightId}/orders           # Create new order
GET    /api/orders/{orderId}                    # Get order status  
POST   /api/orders/{orderId}/seats              # Update seat selection
POST   /api/orders/{orderId}/payment            # Submit payment
DELETE /api/orders/{orderId}                    # Cancel order
GET    /api/flights/{flightId}/seats            # Get available seats
```

#### Temporal Workflows

**Main Workflow: BookingWorkflow**
- Orchestrates the entire booking lifecycle
- Handles seat reservation with 15-minute timer
- Manages timer refresh when seats are updated
- Processes payment with retries
- Handles timeouts and cancellations

**Workflow Features:**
- Signal handlers for:
  - `updateSeats`: Update seat selection (resets timer to 15 minutes)
  - `submitPayment`: Submit payment code for validation
- Query handler for:
  - `getStatus`: Return current booking state (seats, timer, status)
- Timer management:
  - 15-minute countdown that auto-releases seats on expiration
  - Timer cancellation and restart on seat updates

**Child Workflow: PaymentValidationWorkflow**
- Validates 5-digit payment codes
- 10-second timeout per attempt
- Automatic retries (max 3 attempts)
- 15% simulated failure rate

#### Activities (Workers)

**Seat Management Activities:**
```go
- ReserveSeatsActivity(flightID, seats, orderID, userID) error
- ReleaseSeatsActivity(orderID) error  
- UpdateSeatsActivity(orderID, oldSeats, newSeats) error
```

**Payment Activities:**
```go
- ValidatePaymentActivity(paymentCode) (*PaymentResult, error)
  // Simulate 15% failure rate
  // Validate 5-digit format
  // Random delay up to 5 seconds
```

**Order Management Activities:**
```go
- UpdateOrderStatusActivity(orderID, status) error
- SendConfirmationActivity(orderID) error
```

### 3. Database Schema

```sql
-- Orders table
CREATE TABLE orders (
    order_id VARCHAR(36) PRIMARY KEY,
    flight_id VARCHAR(50) NOT NULL,
    user_id VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    workflow_id VARCHAR(100),
    run_id VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_flight_id (flight_id)
);

-- Seats table
CREATE TABLE seats (
    seat_id VARCHAR(36) PRIMARY KEY,
    flight_id VARCHAR(50) NOT NULL,
    seat_number VARCHAR(10) NOT NULL,
    status ENUM('AVAILABLE', 'RESERVED', 'BOOKED') NOT NULL DEFAULT 'AVAILABLE',
    reserved_by VARCHAR(36),
    user_id VARCHAR(50),
    reserved_at TIMESTAMP NULL,
    UNIQUE KEY unique_flight_seat (flight_id, seat_number),
    INDEX idx_flight_status (flight_id, status),
    FOREIGN KEY (reserved_by) REFERENCES orders(order_id) ON DELETE SET NULL
);

-- Payments table
CREATE TABLE payments (
    payment_id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    payment_code VARCHAR(5) NOT NULL,
    transaction_id VARCHAR(100),
    status VARCHAR(20) NOT NULL,
    attempts INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(order_id)
);

-- Seed data: Create one flight with 30 seats (A1-A10, B1-B10, C1-C10)
```

### 4. Business Rules Implementation

**Seat Reservation:**
- Use database row-level locking (`SELECT ... FOR UPDATE`) to prevent race conditions
- 15-minute hold timer starts when seats are reserved
- Timer resets to 15:00 when user updates seat selection
- Expired reservations (>15 minutes) auto-release

**Payment Validation:**
- Accept only 5-digit codes
- Validate within 10 seconds (activity timeout)
- Retry on failure (max 3 attempts)
- Simulate 15% random failure rate: `if rand.Float32() < 0.15 { return error }`
- After 3 failures, order fails and seats are released

**Order States:**
```
CREATED → SEATS_RESERVED → PAYMENT_PENDING → CONFIRMED
                ↓                   ↓
              EXPIRED             FAILED
```

**Concurrency Handling:**
```go
func ReserveSeats(db *sql.DB, flightID string, seats []string, orderID, userID string) error {
    tx, _ := db.Begin()
    defer tx.Rollback()
    
    // Lock rows for update
    query := `
        SELECT seat_id, status, reserved_at
        FROM seats 
        WHERE flight_id = ? AND seat_number IN (?)
        FOR UPDATE
    `
    rows, _ := tx.Query(query, flightID, seats)
    
    // Check availability (including expired reservations)
    for rows.Next() {
        var status string
        var reservedAt sql.NullTime
        
        if status == "AVAILABLE" || 
           (status == "RESERVED" && time.Since(reservedAt.Time) > 15*time.Minute) {
            // OK to reserve
        } else {
            return errors.New("seat not available")
        }
    }
    
    // Reserve seats
    tx.Exec(`UPDATE seats SET status='RESERVED', reserved_by=?, user_id=?, reserved_at=NOW() WHERE ...`)
    tx.Commit()
    return nil
}
```

### 5. Simple Frontend (HTMX)

Create `static/index.html` with:

**Features:**
- Display available seats as a grid (color-coded: available/reserved/booked)
- User ID input field (no authentication needed)
- Click seats to select/deselect
- Submit booking button (starts order and timer)
- Real-time countdown timer display (polls server every second)
- Payment code input (5 digits)
- Order status display
- Cancel booking button

**HTMX Usage:**
```html
<!-- Poll order status every 1 second -->
<div hx-get="/api/orders/{orderId}" 
     hx-trigger="every 1s"
     hx-swap="innerHTML">
  <div class="timer">Time remaining: <span id="countdown"></span></div>
  <div class="status">Status: <span id="status"></span></div>
</div>

<!-- Seat selection -->
<div id="seat-grid" hx-get="/api/flights/FL123/seats" hx-trigger="load">
  <!-- Seats populated here -->
</div>

<!-- Submit payment -->
<form hx-post="/api/orders/{orderId}/payment" hx-swap="outerHTML">
  <input type="text" name="paymentCode" maxlength="5" pattern="[0-9]{5}" required>
  <button type="submit">Pay</button>
</form>
```

**Client-side timer calculation:**
```javascript
// Calculate remaining time from reservation timestamp
function updateTimer(reservedAt) {
    const elapsed = Date.now() - new Date(reservedAt).getTime();
    const remaining = Math.max(0, 15*60*1000 - elapsed);
    const minutes = Math.floor(remaining / 60000);
    const seconds = Math.floor((remaining % 60000) / 1000);
    document.getElementById('countdown').textContent = `${minutes}:${seconds.toString().padStart(2, '0')}`;
}
```

### 6. Docker Setup

**docker-compose.yml:**
```yaml
version: '3.8'

services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: flight_booking
      MYSQL_USER: booking_user
      MYSQL_PASSWORD: booking_pass
    ports:
      - "3306:3306"
    volumes:
      - ./data/mysql:/var/lib/mysql
      - ./scripts/init.sql:/docker-entrypoint-initdb.d/init.sql
    command: --default-authentication-plugin=mysql_native_password

  temporal:
    image: temporalio/auto-setup:latest
    environment:
      - DB=mysql
      - MYSQL_SEEDS=mysql
      - MYSQL_USER=root
      - MYSQL_PWD=rootpass
    ports:
      - "7233:7233"
      - "8080:8080"  # Temporal Web UI
    depends_on:
      - mysql

  temporal-admin-tools:
    image: temporalio/admin-tools:latest
    environment:
      - TEMPORAL_CLI_ADDRESS=temporal:7233
    depends_on:
      - temporal
    stdin_open: true
    tty: true
```

**scripts/init.sql**: Include the schema creation and seed data

### 7. Python Test Suite

**tests/test_booking_flow.py:**

```python
import pytest
import requests
import time
from concurrent.futures import ThreadPoolExecutor

BASE_URL = "http://localhost:8080/api"
FLIGHT_ID = "FL123"

@pytest.fixture
def api_client():
    return requests.Session()

class TestBasicBooking:
    """Test basic booking flow"""
    
    def test_create_order_and_book_seats(self, api_client):
        """Happy path: Create order, select seats, pay successfully"""
        # Create order
        resp = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "alice",
            "seats": ["A1", "A2"]
        })
        assert resp.status_code == 201
        order_id = resp.json()["orderId"]
        
        # Check status
        resp = api_client.get(f"{BASE_URL}/orders/{order_id}")
        assert resp.status_code == 200
        assert resp.json()["status"] == "SEATS_RESERVED"
        assert resp.json()["seats"] == ["A1", "A2"]
        
        # Submit payment
        resp = api_client.post(f"{BASE_URL}/orders/{order_id}/payment", json={
            "paymentCode": "12345"
        })
        assert resp.status_code == 200
        
        # Wait for payment processing
        time.sleep(2)
        
        # Verify confirmation
        resp = api_client.get(f"{BASE_URL}/orders/{order_id}")
        assert resp.json()["status"] in ["CONFIRMED", "PAYMENT_PENDING"]

    def test_invalid_payment_code(self, api_client):
        """Test invalid payment code format"""
        resp = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "bob",
            "seats": ["B1"]
        })
        order_id = resp.json()["orderId"]
        
        # Invalid code (4 digits)
        resp = api_client.post(f"{BASE_URL}/orders/{order_id}/payment", json={
            "paymentCode": "1234"
        })
        assert resp.status_code == 400

class TestConcurrency:
    """Test concurrent booking scenarios"""
    
    def test_concurrent_same_seat_booking(self, api_client):
        """Two users try to book same seat simultaneously"""
        
        def book_seat(user_id):
            resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
                "userId": user_id,
                "seats": ["C1"]
            })
            return resp.status_code, resp.json()
        
        with ThreadPoolExecutor(max_workers=2) as executor:
            future1 = executor.submit(book_seat, "user1")
            future2 = executor.submit(book_seat, "user2")
            
            result1 = future1.result()
            result2 = future2.result()
        
        # One should succeed (201), one should fail (409 or 400)
        status_codes = [result1[0], result2[0]]
        assert 201 in status_codes
        assert 201 not in [s for s in status_codes if s != result1[0] or s != result2[0]]

    def test_seat_update_releases_old_seats(self, api_client):
        """User updates seat selection, old seats become available"""
        # User 1 books A5
        resp1 = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "diana",
            "seats": ["A5"]
        })
        order_id1 = resp1.json()["orderId"]
        
        # User 2 tries A5 (should fail)
        resp2 = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "eve",
            "seats": ["A5"]
        })
        assert resp2.status_code in [400, 409]
        
        # User 1 changes to B5
        resp3 = api_client.post(f"{BASE_URL}/orders/{order_id1}/seats", json={
            "seats": ["B5"]
        })
        assert resp3.status_code == 200
        
        # User 2 tries A5 again (should succeed now)
        resp4 = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "eve",
            "seats": ["A5"]
        })
        assert resp4.status_code == 201

class TestTimerExpiration:
    """Test reservation timer expiration"""
    
    def test_seats_released_after_timeout(self, api_client):
        """Seats are released after 15 minutes (use shorter timer for testing)"""
        # Note: Configure timer to 30 seconds for testing
        resp = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "frank",
            "seats": ["C5"]
        })
        order_id = resp.json()["orderId"]
        
        # Wait for timer to expire (31 seconds if configured to 30s)
        time.sleep(31)
        
        # Check order status
        resp = api_client.get(f"{BASE_URL}/orders/{order_id}")
        assert resp.json()["status"] == "EXPIRED"
        
        # Verify seat is available again
        resp = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "grace",
            "seats": ["C5"]
        })
        assert resp.status_code == 201

    def test_timer_refresh_on_seat_update(self, api_client):
        """Timer resets when user updates seats"""
        resp = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "henry",
            "seats": ["A7"]
        })
        order_id = resp.json()["orderId"]
        
        # Wait 20 seconds
        time.sleep(20)
        
        # Update seats (should reset timer)
        resp = api_client.post(f"{BASE_URL}/orders/{order_id}/seats", json={
            "seats": ["A8"]
        })
        assert resp.status_code == 200
        
        # Check that reservation is still active after total 25 seconds
        time.sleep(5)
        resp = api_client.get(f"{BASE_URL}/orders/{order_id}")
        assert resp.json()["status"] != "EXPIRED"

class TestPaymentRetry:
    """Test payment retry logic"""
    
    def test_payment_retries_on_failure(self, api_client):
        """Payment should retry up to 3 times on failure"""
        resp = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "iris",
            "seats": ["B7"]
        })
        order_id = resp.json()["orderId"]
        
        # Submit payment (may fail with 15% probability)
        resp = api_client.post(f"{BASE_URL}/orders/{order_id}/payment", json={
            "paymentCode": "99999"
        })
        
        # Wait for retries to complete
        time.sleep(35)  # 10s timeout × 3 attempts + buffer
        
        # Check final status
        resp = api_client.get(f"{BASE_URL}/orders/{order_id}")
        status = resp.json()["status"]
        assert status in ["CONFIRMED", "FAILED"]

class TestOrderCancellation:
    """Test order cancellation"""
    
    def test_cancel_order_releases_seats(self, api_client):
        """Cancelling order releases reserved seats"""
        resp = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "jack",
            "seats": ["C7"]
        })
        order_id = resp.json()["orderId"]
        
        # Cancel order
        resp = api_client.delete(f"{BASE_URL}/orders/{order_id}")
        assert resp.status_code == 200
        
        # Verify seat is available
        resp = api_client.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "kate",
            "seats": ["C7"]
        })
        assert resp.status_code == 201
```

**tests/conftest.py:**
```python
import pytest
import requests
import time

@pytest.fixture(scope="session", autouse=True)
def wait_for_services():
    """Wait for services to be ready before running tests"""
    max_retries = 30
    for i in range(max_retries):
        try:
            resp = requests.get("http://localhost:8080/health")
            if resp.status_code == 200:
                return
        except requests.ConnectionError:
            pass
        time.sleep(1)
    pytest.fail("Services did not start in time")

@pytest.fixture(scope="function")
def reset_flight_seats():
    """Reset flight seats before each test"""
    # Call admin endpoint to reset seats (implement this)
    requests.post("http://localhost:8080/api/admin/reset")
    yield
```

**pytest.ini:**
```ini
[pytest]
testpaths = tests
python_files = test_*.py
python_classes = Test*
python_functions = test_*
markers =
    slow: marks tests as slow
    integration: marks tests as integration tests
```

**requirements.txt:**
```
pytest==7.4.3
requests==2.31.0
pytest-xdist==3.5.0  # For parallel test execution
```

### 8. Project Structure

```
flight-booking-system/
├── cmd/
│   ├── server/
│   │   └── main.go              # HTTP server entry point
│   └── worker/
│       └── main.go              # Temporal worker entry point
├── internal/
│   ├── api/
│   │   ├── handlers.go          # HTTP handlers
│   │   ├── middleware.go        # CORS, logging
│   │   └── router.go            # Route definitions
│   ├── database/
│   │   ├── db.go                # Database connection
│   │   └── queries.go           # SQL queries
│   ├── models/
│   │   └── models.go            # Data structures
│   ├── temporal/
│   │   ├── workflows/
│   │   │   ├── booking.go       # BookingWorkflow
│   │   │   └── payment.go       # PaymentValidationWorkflow
│   │   └── activities/
│   │       ├── seats.go         # Seat management activities
│   │       ├── payment.go       # Payment activities
│   │       └── orders.go        # Order management activities
│   └── config/
│       └── config.go            # Configuration
├── static/
│   ├── index.html               # Frontend UI
│   ├── styles.css
│   └── app.js
├── scripts/
│   └── init.sql                 # Database initialization
├── tests/
│   ├── conftest.py
│   ├── test_booking_flow.py
│   ├── test_concurrency.py
│   ├── test_timer.py
│   └── test_payment.py
├── docker-compose.yml
├── Makefile
├── go.mod
├── go.sum
├── README.md
└── requirements.txt             # Python test dependencies
```

### 9. Makefile

```makefile
.PHONY: setup start stop test clean

setup:
	@echo "Setting up project..."
	docker-compose up -d mysql
	sleep 10
	docker-compose up -d temporal
	sleep 5
	go mod download
	pip install -r requirements.txt

start:
	@echo "Starting services..."
	docker-compose up -d
	sleep 5
	go run cmd/server/main.go &
	go run cmd/worker/main.go &

stop:
	@echo "Stopping services..."
	pkill -f "go run"
	docker-compose down

test:
	@echo "Running tests..."
	pytest tests/ -v --tb=short

test-parallel:
	@echo "Running tests in parallel..."
	pytest tests/ -v -n 4

clean:
	@echo "Cleaning up..."
	docker-compose down -v
	rm -rf data/
	pkill -f "go run"

logs:
	docker-compose logs -f

db-shell:
	docker-compose exec mysql mysql -u booking_user -pbooking_pass flight_booking
```

### 10. Implementation Guidelines

**Go Server Configuration:**
```go
// internal/config/config.go
type Config struct {
    ServerPort      string
    DatabaseDSN     string
    TemporalAddress string
    ReservationTimeout time.Duration  // Default: 15 minutes (use 30s for testing)
    PaymentTimeout     time.Duration  // Default: 10 seconds
}

// Load from environment or use defaults
func Load() *Config {
    return &Config{
        ServerPort:         getEnv("SERVER_PORT", "8080"),
        DatabaseDSN:        getEnv("DATABASE_DSN", "booking_user:booking_pass@tcp(localhost:3306)/flight_booking"),
        TemporalAddress:    getEnv("TEMPORAL_ADDRESS", "localhost:7233"),
        ReservationTimeout: parseDuration(getEnv("RESERVATION_TIMEOUT", "15m")),
        PaymentTimeout:     parseDuration(getEnv("PAYMENT_TIMEOUT", "10s")),
    }
}
```

**Workflow State Management:**
```go
type BookingState struct {
    OrderID            string
    FlightID           string
    UserID             string
    Seats              []string
    Status             string
    ReservationStartAt time.Time
    PaymentAttempts    int
}
```

**Signal and Query Handlers:**
```go
func BookingWorkflow(ctx workflow.Context, input BookingInput) (*BookingResult, error) {
    state := &BookingState{
        OrderID:  input.OrderID,
        FlightID: input.FlightID,
        UserID:   input.UserID,
        Status:   "CREATED",
    }
    
    // Query handler for real-time status
    err := workflow.SetQueryHandler(ctx, "getStatus", func() (*BookingState, error) {
        return state, nil
    })
    
    // Signal channels
    seatUpdateChan := workflow.GetSignalChannel(ctx, "updateSeats")
    paymentChan := workflow.GetSignalChannel(ctx, "submitPayment")
    
    // Reserve initial seats
    activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
    })
    
    err = workflow.ExecuteActivity(activityCtx, ReserveSeatsActivity, 
        input.FlightID, input.Seats, input.OrderID, input.UserID).Get(ctx, nil)
    if err != nil {
        return nil, err
    }
    
    state.Seats = input.Seats
    state.Status = "SEATS_RESERVED"
    state.ReservationStartAt = workflow.Now(ctx)
    
    // Start 15-minute timer
    timerFuture := workflow.NewTimer(ctx, 15*time.Minute)
    
    // Main event loop
    for {
        selector := workflow.NewSelector(ctx)
        
        // Handle seat updates
        selector.AddReceive(seatUpdateChan, func(c workflow.ReceiveChannel, more bool) {
            var newSeats []string
            c.Receive(ctx, &newSeats)
            
            // Update seats
            workflow.ExecuteActivity(activityCtx, UpdateSeatsActivity, 
                state.OrderID, state.Seats, newSeats).Get(ctx, nil)
            
            state.Seats = newSeats
            state.ReservationStartAt = workflow.Now(ctx)
            
            // Cancel old timer and start new one
            timerFuture = workflow.NewTimer(ctx, 15*time.Minute)
        })
        
        // Handle payment submission
        selector.AddReceive(paymentChan, func(c workflow.ReceiveChannel, more bool) {
            var paymentCode string
            c.Receive(ctx, &paymentCode)
            
            state.Status = "PAYMENT_PENDING"
            
            // Execute payment validation child workflow
            childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
                WorkflowID: state.OrderID + "-payment",
            })
            
            var paymentResult PaymentResult
            err := workflow.ExecuteChildWorkflow(childCtx, PaymentValidationWorkflow, 
                paymentCode, state.OrderID).Get(ctx, &paymentResult)
            
            if err == nil && paymentResult.Success {
                state.Status = "CONFIRMED"
                workflow.ExecuteActivity(activityCtx, UpdateOrderStatusActivity, 
                    state.OrderID, "CONFIRMED")
                workflow.ExecuteActivity(activityCtx, SendConfirmationActivity, state.OrderID)
            } else {
                state.Status = "FAILED"
                workflow.ExecuteActivity(activityCtx, ReleaseSeatsActivity, state.OrderID)
                workflow.ExecuteActivity(activityCtx, UpdateOrderStatusActivity, 
                    state.OrderID, "FAILED")
            }
        })
        
        // Handle timer expiration
        selector.AddFuture(timerFuture, func(f workflow.Future) {
            state.Status = "EXPIRED"
            workflow.ExecuteActivity(activityCtx, ReleaseSeatsActivity, state.OrderID)
            workflow.ExecuteActivity(activityCtx, UpdateOrderStatusActivity, 
                state.OrderID, "EXPIRED")
        })
        
        selector.Select(ctx)
        
        // Exit conditions
        if state.Status == "CONFIRMED" || state.Status == "FAILED" || state.Status == "EXPIRED" {
            break
        }
    }
    
    return &BookingResult{State: state}, nil
}
```

**API Handler Example:**
```go
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
    var req CreateOrderRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Validate
    if req.UserID == "" || len(req.Seats) == 0 {
        http.Error(w, "userId and seats required", http.StatusBadRequest)
        return
    }
    
    orderID := uuid.New().String()
    flightID := mux.Vars(r)["flightId"]
    
    // Start Temporal workflow
    workflowOptions := client.StartWorkflowOptions{
        ID:        orderID,
        TaskQueue: "booking-task-queue",
    }
    
    input := BookingInput{
        OrderID:  orderID,
        FlightID: flightID,
        UserID:   req.UserID,
        Seats:    req.Seats,
    }
    
    we, err := h.temporalClient.ExecuteWorkflow(context.Background(), workflowOptions, 
        BookingWorkflow, input)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Store workflow info in database
    h.db.Exec(`
        INSERT INTO orders (order_id, flight_id, user_id, status, workflow_id, run_id)
        VALUES (?, ?, ?, 'CREATED', ?, ?)
    `, orderID, flightID, req.UserID, we.GetID(), we.GetRunID())
    
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(CreateOrderResponse{
        OrderID:    orderID,
        FlightID:   flightID,
        UserID:     req.UserID,
        Seats:      req.Seats,
        Status:     "CREATED",
        WorkflowID: we.GetID(),
    })
}
```

### 11. Testing Configuration

**Environment Variables for Testing:**
```bash
# .env.test
RESERVATION_TIMEOUT=30s  # Short timeout for faster tests
PAYMENT_TIMEOUT=5s
SERVER_PORT=8080
```

**Test Execution:**
```bash
# Run all tests
make test

# Run specific test class
pytest tests/test_booking_flow.py::TestBasicBooking -v

# Run with coverage
pytest tests/ --cov=. --cov-report=html

# Run in parallel
make test-parallel
```

### 12. README Requirements

Include in README.md:
- Architecture diagram explanation
- Setup instructions (Docker, Go, Python)
- How to run the system
- API documentation with curl examples
- Test execution instructions
- Multi-user testing examples
- Business rules explanation
- Known limitations/assumptions

### 13. Key Success Criteria

✅ Temporal workflow orchestrates entire booking flow  
✅ 15-minute reservation timer with auto-release  
✅ Timer resets when seats are updated  
✅ Payment validation with 10s timeout and 3 retries  
✅ 15% payment failure simulation  
✅ Concurrent booking handling (no double-booking)  
✅ Database transactions prevent race conditions  
✅ Real-time status via polling (1s intervals)  
✅ Simple HTMX UI for testing  
✅ Comprehensive Python test suite  
✅ Docker-based local environment  
✅ File-based MySQL persistence  

### 14. Optional Enhancements (if time permits)

- Temporal Web UI integration for workflow visualization
- Prometheus metrics export
- Structured logging (zerolog)
- Graceful shutdown handling
- Health check endpoints
- Admin endpoints (reset flight, view all orders)
- Seat map visualization in UI
- Multiple flights support

---

## Getting Started Command Sequence

```bash
# 1. Setup
make setup

# 2. Start services
make start

# 3. Open UI
open http://localhost:8080

# 4. Run tests
make test

# 5. View Temporal UI
open http://localhost:8080  # Temporal Web UI

# 6. Check logs
make logs
```

This should produce a fully functional flight booking system demonstrating Temporal workflow orchestration, concurrent booking handling, timeout management, and payment retry logic.
