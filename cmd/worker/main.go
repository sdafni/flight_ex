package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"flight-booking-system/internal/config"
	"flight-booking-system/internal/database"
	"flight-booking-system/internal/temporal/activities"
	"flight-booking-system/internal/temporal/workflows"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := database.NewDB(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Connected to database")

	// Connect to Temporal
	temporalClient, err := client.Dial(client.Options{
		HostPort: cfg.TemporalAddress,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer temporalClient.Close()

	log.Println("Connected to Temporal")

	// Create worker
	w := worker.New(temporalClient, "booking-task-queue", worker.Options{})

	// Register workflows
	w.RegisterWorkflow(workflows.BookingWorkflow)
	w.RegisterWorkflow(workflows.PaymentValidationWorkflow)

	// Register activities
	seatActivities := activities.NewSeatActivities(db)
	w.RegisterActivity(seatActivities.ReserveSeats)
	w.RegisterActivity(seatActivities.ReleaseSeats)
	w.RegisterActivity(seatActivities.UpdateSeats)
	w.RegisterActivity(seatActivities.ConfirmSeats)

	paymentActivities := activities.NewPaymentActivities(db)
	w.RegisterActivity(paymentActivities.ValidatePayment)
	w.RegisterActivity(paymentActivities.UpdatePaymentRecord)

	orderActivities := activities.NewOrderActivities(db)
	w.RegisterActivity(orderActivities.UpdateOrderStatus)
	w.RegisterActivity(orderActivities.SendConfirmation)

	// Start worker
	err = w.Start()
	if err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	log.Println("Worker started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down worker...")
	w.Stop()
	log.Println("Worker stopped")
}
