# Flight Booking System with Temporal

A Temporal demostration flight booking system built with Go 
 demonstrating workflow orchestration, seat reservations with timeouts, payment validation with retries, and concurrent booking handling.

## Features

- 🎯 **Temporal Workflows** - Complete booking lifecycle orchestration
- ⏰ **15-Minute Timer** - Auto-releases seats on expiration, resets on updates
- 💳 **Payment Retries** - 10s timeout, 3 attempts, 15% simulated failure
- 🔒 **Concurrency Safe** - Row-level locking prevents double-booking
- 🎨 **Interactive UI** - HTMX-powered real-time seat selection
- 🧪 **Comprehensive Tests** - Python pytest suite with 12+ scenarios

## Quick Start

```bash
make setup      # First time only, setp tests
make start      # Start all services
make test       # Run tests
```

**URLs**: http://localhost:8080 (app) | http://localhost:8088 (Temporal UI)

## Architecture

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│  Browser    │─────▶│  Go Server   │─────▶│  Temporal   │
│  (HTMX)     │◀─────│  (REST API)  │◀─────│  Workflows  │
└─────────────┘      └──────────────┘      └─────────────┘
                            │                      │
                            ▼                      ▼
                     ┌──────────────┐      ┌─────────────┐
                     │   MySQL      │◀─────│ Activities  │
                     └──────────────┘      └─────────────┘
```

**Tech Stack**: Go, Temporal, MySQL, HTMX, Python (tests)

## Project Structure

```
flight-booking-system/
├── cmd/
│   ├── server/          # HTTP server
│   └── worker/          # Temporal worker
├── internal/
│   ├── api/            # REST handlers
│   ├── database/       # MySQL queries
│   ├── models/         # Data structures
│   └── temporal/       # Workflows & activities
├── static/             # Frontend (HTML/CSS/JS)
├── scripts/init.sql    # Database schema
├── tests/              # Python test suite
└── docker-compose.yml  # MySQL + Temporal
```

## API Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/flights/{id}/orders` | Create booking |
| GET | `/api/orders/{id}` | Get order status |
| POST | `/api/orders/{id}/seats` | Update seats |
| POST | `/api/orders/{id}/payment` | Submit payment |
| DELETE | `/api/orders/{id}` | Cancel order |
| GET | `/api/flights/{id}/seats` | Get available seats |

## Configuration

Environment variables (defaults shown):
```bash
SERVER_PORT=8080
DATABASE_DSN=booking_user:booking_pass@tcp(localhost:3306)/flight_booking
TEMPORAL_ADDRESS=localhost:7233
RESERVATION_TIMEOUT=15m      # Use 30s for testing
PAYMENT_TIMEOUT=10s
MAX_PAYMENT_RETRIES=3
```

## Business Rules

### Seat Reservation
- 15-minute hold timer (auto-release on expiration)
- Timer resets when seats updated
- Database row-level locking (`SELECT ... FOR UPDATE`)
- Max 30 seats per flight (A1-A10, B1-B10, C1-C10)

### Payment Processing
- 5-digit code validation
- 10-second timeout per attempt
- Max 3 retries with exponential backoff
- 15% simulated failure rate
- Order fails after 3 failed attempts

### Order States
```
CREATED → SEATS_RESERVED → PAYMENT_PENDING → CONFIRMED
            ↓                     ↓
         EXPIRED              FAILED/CANCELLED
```



## Development

**Prerequisites**: Docker, Go 1.21+, Python 3.9+

```bash
make build       # Build binaries (bin/server, bin/worker)
```

## Key Workflows

### BookingWorkflow (Main)
- Orchestrates entire booking lifecycle
- Manages 15-minute seat reservation timer
- Handles signals: `updateSeats`, `submitPayment`, `cancelOrder`
- Query: `getStatus` returns real-time state

### PaymentValidationWorkflow (Child)
- Validates 5-digit payment codes
- 10-second timeout per attempt
- Automatic retries (max 3)
- 15% simulated failure rate

## Production Considerations

### Implemented
- ✅ Graceful shutdown
- ✅ Transaction management
- ✅ Connection pooling
- ✅ Error handling
- ✅ Logging
- ✅ Healthchecks

### Not Implemented (Future)
- Authentication/authorization
- Real payment gateway
- Email/SMS notifications
- Multiple flights
- Metrics/monitoring
- Rate limiting

## TODO / Known Issues
- [ ] Remove `config.Load()` from workflows (non-deterministic) - pass timeouts via workflow input
- [ ] Fix `GetOrder` to return `database.ErrOrderNotFound` instead of generic error
- [ ] Remove `time.Sleep(100ms)` from `CreateOrder` handler - race condition
- [ ] Replace hard-coded `15*time.Minute` in `reserveSeatsInTx` with config parameter
- [ ] Fix order creation race: persist order before starting workflow, or cancel workflow on DB failure
- [ ] Use `r.Context()` instead of `context.Background()` for Temporal/DB calls
- [ ] Standardize error responses to JSON format (replace `http.Error` with JSON helper)
- [ ] Pass `*config.Config` to handler constructor instead of loading per request
- [ ] Make CORS origins configurable (currently allows `*`)
- [ ] API calls don't properly handle requests when order is in terminal state (CONFIRMED/FAILED/EXPIRED/CANCELLED)
  - Fix API calls handling when order is closed (or just tests)
- [ ] Centralize reservation timeout logic - DB and workflow use different sources
- [ ] Check activity errors for `UpdateOrderStatus`/`UpdatePaymentRecord` - currently ignored
- [ ] Clarify payment result/error handling - currently mixes `error` and `PaymentResult.Success`
- [ ] Activities receive multiple individual arguments instead of structured objects
  - Pass arguments as single objects to activities
- [ ] Add context support to database methods for cancellation/timeout propagation

## License

MIT
