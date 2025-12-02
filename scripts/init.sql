-- Flight Booking System Database Schema

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    order_id VARCHAR(36) PRIMARY KEY,
    flight_id VARCHAR(50) NOT NULL,
    user_id VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    workflow_id VARCHAR(100),
    run_id VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_flight_id (flight_id),
    INDEX idx_workflow_id (workflow_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Seats table
CREATE TABLE IF NOT EXISTS seats (
    seat_id VARCHAR(36) PRIMARY KEY,
    flight_id VARCHAR(50) NOT NULL,
    seat_number VARCHAR(10) NOT NULL,
    status ENUM('AVAILABLE', 'RESERVED', 'BOOKED') NOT NULL DEFAULT 'AVAILABLE',
    reserved_by VARCHAR(36),
    user_id VARCHAR(50),
    reserved_at TIMESTAMP NULL,
    UNIQUE KEY unique_flight_seat (flight_id, seat_number),
    INDEX idx_flight_status (flight_id, status),
    INDEX idx_reserved_by (reserved_by),
    FOREIGN KEY (reserved_by) REFERENCES orders(order_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Payments table
CREATE TABLE IF NOT EXISTS payments (
    payment_id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    payment_code VARCHAR(5) NOT NULL,
    transaction_id VARCHAR(100),
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_order_id (order_id),
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Seed data: Create one flight with 30 seats (A1-A10, B1-B10, C1-C10)
INSERT INTO seats (seat_id, flight_id, seat_number, status) VALUES
-- Row A (1-10)
(UUID(), 'FL123', 'A1', 'AVAILABLE'),
(UUID(), 'FL123', 'A2', 'AVAILABLE'),
(UUID(), 'FL123', 'A3', 'AVAILABLE'),
(UUID(), 'FL123', 'A4', 'AVAILABLE'),
(UUID(), 'FL123', 'A5', 'AVAILABLE'),
(UUID(), 'FL123', 'A6', 'AVAILABLE'),
(UUID(), 'FL123', 'A7', 'AVAILABLE'),
(UUID(), 'FL123', 'A8', 'AVAILABLE'),
(UUID(), 'FL123', 'A9', 'AVAILABLE'),
(UUID(), 'FL123', 'A10', 'AVAILABLE'),
-- Row B (1-10)
(UUID(), 'FL123', 'B1', 'AVAILABLE'),
(UUID(), 'FL123', 'B2', 'AVAILABLE'),
(UUID(), 'FL123', 'B3', 'AVAILABLE'),
(UUID(), 'FL123', 'B4', 'AVAILABLE'),
(UUID(), 'FL123', 'B5', 'AVAILABLE'),
(UUID(), 'FL123', 'B6', 'AVAILABLE'),
(UUID(), 'FL123', 'B7', 'AVAILABLE'),
(UUID(), 'FL123', 'B8', 'AVAILABLE'),
(UUID(), 'FL123', 'B9', 'AVAILABLE'),
(UUID(), 'FL123', 'B10', 'AVAILABLE'),
-- Row C (1-10)
(UUID(), 'FL123', 'C1', 'AVAILABLE'),
(UUID(), 'FL123', 'C2', 'AVAILABLE'),
(UUID(), 'FL123', 'C3', 'AVAILABLE'),
(UUID(), 'FL123', 'C4', 'AVAILABLE'),
(UUID(), 'FL123', 'C5', 'AVAILABLE'),
(UUID(), 'FL123', 'C6', 'AVAILABLE'),
(UUID(), 'FL123', 'C7', 'AVAILABLE'),
(UUID(), 'FL123', 'C8', 'AVAILABLE'),
(UUID(), 'FL123', 'C9', 'AVAILABLE'),
(UUID(), 'FL123', 'C10', 'AVAILABLE')
ON DUPLICATE KEY UPDATE status=status;
