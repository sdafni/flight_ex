package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

func NewRouter(h *Handler) *mux.Router {
	r := mux.NewRouter()

	// Apply middleware
	r.Use(CORSMiddleware)
	r.Use(LoggingMiddleware)

	// API routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(JSONMiddleware)

	// Health check
	r.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// Flight routes
	api.HandleFunc("/flights/{flightId}/orders", h.CreateOrder).Methods("POST")
	api.HandleFunc("/flights/{flightId}/seats", h.GetSeats).Methods("GET")

	// Order routes
	api.HandleFunc("/orders/{orderId}", h.GetOrderStatus).Methods("GET")
	api.HandleFunc("/orders/{orderId}/seats", h.UpdateSeats).Methods("POST")
	api.HandleFunc("/orders/{orderId}/payment", h.SubmitPayment).Methods("POST")
	api.HandleFunc("/orders/{orderId}", h.CancelOrder).Methods("DELETE")

	// Admin routes (for testing)
	api.HandleFunc("/admin/flights/{flightId}/reset", h.ResetFlight).Methods("POST")

	// Serve static files
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))

	return r
}
