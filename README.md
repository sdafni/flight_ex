# Flight Booking System with Temporal

A production-ready flight booking system built with Go and Temporal demonstrating workflow orchestration, seat reservations with timeouts, payment validation with retries, and concurrent booking handling.

## Features

- 🎯 **Temporal Workflows** - Complete booking lifecycle orchestration
- ⏰ **15-Minute Timer** - Auto-releases seats on expiration, resets on updates
- 💳 **Payment Retries** - 10s timeout, 3 attempts, 15% simulated failure
- 🔒 **Concurrency Safe** - Row-level locking prevents double-booking
- 🎨 **Interactive UI** - HTMX-powered real-time seat selection
- 🧪 **Comprehensive Tests** - Python pytest suite with 12+ scenarios

## Quick Start

```bash
# 1. Setup (first time only - creates venv, starts Docker)
make setup

# 2. Start all services
make start

# 3. Open the application
open http://localhost:8080        # Your booking app
open http://localhost:8088        # Temporal UI

# 4. Run tests
make test
```

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

## Testing

```bash
# Run all tests (uses virtual environment)
make test

# Run specific test
source venv/bin/activate
pytest tests/test_booking_flow.py::TestBasicBooking -v

# Run in parallel
make test-parallel
```

**Test Coverage**: Basic flow, concurrency, timeouts, retries, cancellation

## Development

### Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Python 3.9+
- Make (optional)

### Manual Setup
```bash
# Start Docker services
docker-compose up -d
sleep 60  # Wait for Temporal to initialize

# Create Python venv
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt

# Install Go dependencies
go mod download

# Start worker
go run cmd/worker/main.go &

# Start server
go run cmd/server/main.go &
```

### Building
```bash
make build  # Creates bin/server and bin/worker
```

### Database Access
```bash
make db-shell  # Opens MySQL shell
```

## Docker Services

**4 containers** (3 required + 1 optional):
1. **MySQL** (port 3306) - Database
2. **Temporal Server** (port 7233) - Workflow engine
3. **Temporal UI** (port 8088) - Web interface
4. **Admin Tools** (optional) - CLI tools

```bash
# Check status
docker-compose ps

# View logs
docker-compose logs -f temporal

# Restart
docker-compose restart

# Clean everything
docker-compose down -v && rm -rf data/
```

**Note**: Temporal takes 30-60 seconds to initialize on first start.

## Common Commands

```bash
make setup          # Initial setup
make start          # Start all services
make stop           # Stop all services
make test           # Run tests
make build          # Build binaries
make clean          # Remove everything
make logs           # View Docker logs
make db-shell       # MySQL shell
make help           # Show all commands
```

## Usage Examples

### Create Booking (CLI)
```bash
# Create order
curl -X POST http://localhost:8080/api/flights/FL123/orders \
  -H "Content-Type: application/json" \
  -d '{"userId": "alice", "seats": ["A1", "A2"]}'

# Get status
curl http://localhost:8080/api/orders/{orderId}

# Submit payment
curl -X POST http://localhost:8080/api/orders/{orderId}/payment \
  -H "Content-Type: application/json" \
  -d '{"paymentCode": "12345"}'
```

### Update Seats
```bash
curl -X POST http://localhost:8080/api/orders/{orderId}/seats \
  -H "Content-Type: application/json" \
  -d '{"seats": ["B1", "B2"]}'
```

### Cancel Order
```bash
curl -X DELETE http://localhost:8080/api/orders/{orderId}
```

## Troubleshooting

### Temporal not starting
```bash
# Wait longer (normal behavior)
sleep 60
curl http://localhost:8088

# Or check logs
docker-compose logs temporal
```

### Port conflicts
```bash
# Check what's using ports
lsof -i :3306  # MySQL
lsof -i :7233  # Temporal
lsof -i :8088  # Temporal UI
lsof -i :8080  # Your API
```

### Complete reset
```bash
make clean
make setup
```

### Python venv issues
```bash
# Recreate venv
rm -rf venv
make setup
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

## Contributing

1. Fork the repository
2. Create feature branch
3. Make changes with tests
4. Run `make test`
5. Submit pull request

## License

MIT

## Support

- **Issues**: GitHub Issues
- **Temporal Docs**: https://docs.temporal.io/
- **Test Examples**: See `tests/` directory

---

**Quick Reference:**
- Frontend: http://localhost:8080
- Temporal UI: http://localhost:8088
- API: http://localhost:8080/api
- Health: http://localhost:8080/health
