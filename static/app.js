let selectedSeats = new Set();
let currentOrderId = null;
let statusPollInterval = null;
let isUpdatingSeats = false;

// Handle seat grid response from HTMX
document.body.addEventListener('htmx:afterSwap', function(event) {
    if (event.detail.target.id === 'seat-grid') {
        renderSeats(event.detail.xhr.response);
    }
});

// Render seat grid
function renderSeats(responseText) {
    try {
        const data = JSON.parse(responseText);
        const seatGrid = document.getElementById('seat-grid');
        seatGrid.innerHTML = '';

        data.seats.forEach(seat => {
            const seatDiv = document.createElement('div');
            seatDiv.className = 'seat';
            seatDiv.textContent = seat.seatNumber;

            // Determine seat status
            if (seat.status === 'AVAILABLE') {
                seatDiv.classList.add('available');
                seatDiv.onclick = () => toggleSeat(seat.seatNumber);
            } else if (seat.status === 'RESERVED') {
                // Check if reserved by current user
                if (currentOrderId && seat.reservedBy === currentOrderId) {
                    // In update mode, treat reserved seats like available seats
                    if (isUpdatingSeats) {
                        // Only add to selectedSeats if user has explicitly selected it
                        if (selectedSeats.has(seat.seatNumber)) {
                            seatDiv.classList.add('selected');
                        } else {
                            seatDiv.classList.add('available');
                        }
                        seatDiv.onclick = () => toggleSeat(seat.seatNumber);
                    } else {
                        // Not in update mode, show as selected and add to set
                        seatDiv.classList.add('selected');
                        selectedSeats.add(seat.seatNumber);
                    }
                } else {
                    seatDiv.classList.add('reserved');
                }
            } else if (seat.status === 'BOOKED') {
                seatDiv.classList.add('booked');
            }

            // If seat is selected but not yet reserved, mark as selected
            if (selectedSeats.has(seat.seatNumber) && seat.status === 'AVAILABLE') {
                seatDiv.classList.add('selected');
                seatDiv.classList.remove('available');
            }

            seatGrid.appendChild(seatDiv);
        });

        updateSelectedSeatsDisplay();
    } catch (error) {
        console.error('Error parsing seats response:', error);
    }
}

// Toggle seat selection
function toggleSeat(seatNumber) {
    if (currentOrderId && !isUpdatingSeats) {
        return; // Can't select seats if order is active and not updating
    }

    if (selectedSeats.has(seatNumber)) {
        selectedSeats.delete(seatNumber);
    } else {
        selectedSeats.add(seatNumber);
    }

    updateSelectedSeatsDisplay();
    // Trigger seat grid refresh
    htmx.trigger('#seat-grid', 'load');
}

// Update selected seats display
function updateSelectedSeatsDisplay() {
    const seatsList = document.getElementById('selected-seats-list');
    const bookBtn = document.getElementById('book-btn');

    if (selectedSeats.size === 0) {
        seatsList.textContent = 'None';
        bookBtn.disabled = true;
    } else {
        seatsList.textContent = Array.from(selectedSeats).sort().join(', ');
        const userId = document.getElementById('userId').value;
        bookBtn.disabled = !userId || currentOrderId !== null;
    }
}

// Create booking
async function createBooking() {
    const userId = document.getElementById('userId').value;

    if (!userId) {
        alert('Please enter a user ID');
        return;
    }

    if (selectedSeats.size === 0) {
        alert('Please select at least one seat');
        return;
    }

    try {
        const response = await fetch('/api/flights/FL123/orders', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                userId: userId,
                seats: Array.from(selectedSeats)
            })
        });

        if (!response.ok) {
            const error = await response.text();
            alert('Failed to create booking: ' + error);
            return;
        }

        const data = await response.json();
        currentOrderId = data.orderId;

        // Show order section
        document.getElementById('order-section').style.display = 'block';
        document.getElementById('order-id').textContent = data.orderId;

        // Disable booking button
        document.getElementById('book-btn').disabled = true;

        // Start polling for order status
        startStatusPolling();

    } catch (error) {
        alert('Error creating booking: ' + error.message);
    }
}

// Start polling for order status
function startStatusPolling() {
    if (statusPollInterval) {
        clearInterval(statusPollInterval);
    }

    statusPollInterval = setInterval(updateOrderStatus, 1000);
    updateOrderStatus(); // Initial call
}

// Update order status
async function updateOrderStatus() {
    if (!currentOrderId) return;

    try {
        const response = await fetch(`/api/orders/${currentOrderId}`);

        if (!response.ok) {
            console.error('Failed to get order status');
            return;
        }

        const data = await response.json();
        updateOrderUI(data);

    } catch (error) {
        console.error('Error getting order status:', error);
    }
}

// Update order UI based on status
function updateOrderUI(orderData) {
    document.getElementById('order-status').textContent = orderData.status;

    // Update countdown
    if (orderData.timeRemaining > 0 && orderData.status === 'SEATS_RESERVED') {
        const minutes = Math.floor(orderData.timeRemaining / 60);
        const seconds = orderData.timeRemaining % 60;
        document.getElementById('countdown').textContent =
            `${minutes}:${seconds.toString().padStart(2, '0')}`;
    } else {
        document.getElementById('countdown').textContent = '00:00';
    }

    // Show/hide sections based on status
    if (orderData.status === 'SEATS_RESERVED') {
        document.getElementById('update-seats-section').style.display = 'block';
        document.getElementById('payment-section').style.display = 'block';
        document.getElementById('cancel-section').style.display = 'block';
        document.getElementById('new-booking-section').style.display = 'none';
    } else if (orderData.status === 'PAYMENT_PENDING') {
        document.getElementById('update-seats-section').style.display = 'none';
        document.getElementById('payment-section').style.display = 'none';
        document.getElementById('cancel-section').style.display = 'none';
    } else if (orderData.status === 'CONFIRMED') {
        stopStatusPolling();
        document.getElementById('update-seats-section').style.display = 'none';
        document.getElementById('payment-section').style.display = 'none';
        document.getElementById('cancel-section').style.display = 'none';
        document.getElementById('new-booking-section').style.display = 'block';
        alert('Booking confirmed! Your seats are booked.');
    } else if (orderData.status === 'FAILED' || orderData.status === 'EXPIRED' || orderData.status === 'CANCELLED') {
        stopStatusPolling();
        document.getElementById('update-seats-section').style.display = 'none';
        document.getElementById('payment-section').style.display = 'none';
        document.getElementById('cancel-section').style.display = 'none';
        document.getElementById('new-booking-section').style.display = 'block';

        if (orderData.status === 'EXPIRED') {
            alert('Booking expired. Please try again.');
        } else if (orderData.status === 'FAILED') {
            alert('Booking failed. Please try again.');
        } else if (orderData.status === 'CANCELLED') {
            alert('Booking cancelled.');
        }
    }
}

// Enable seat update mode
function enableSeatUpdate() {
    isUpdatingSeats = true;
    selectedSeats.clear();
    document.getElementById('update-seats-btn').style.display = 'inline-block';
    // Refresh seat grid to reflect the cleared selection
    htmx.trigger('#seat-grid', 'load');
    alert('Select new seats, then click "Update Seats"');
}

// Update seats
async function updateSeats() {
    if (selectedSeats.size === 0) {
        alert('Please select at least one seat');
        return;
    }

    try {
        const response = await fetch(`/api/orders/${currentOrderId}/seats`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                seats: Array.from(selectedSeats)
            })
        });

        if (!response.ok) {
            const error = await response.text();
            alert('Failed to update seats: ' + error);
            return;
        }

        isUpdatingSeats = false;
        document.getElementById('update-seats-btn').style.display = 'none';
        alert('Seats updated successfully!');

    } catch (error) {
        alert('Error updating seats: ' + error.message);
    }
}

// Submit payment
async function submitPayment() {
    const paymentCode = document.getElementById('payment-code').value;

    if (!paymentCode || paymentCode.length !== 5) {
        alert('Please enter a valid 5-digit payment code');
        return;
    }

    try {
        const response = await fetch(`/api/orders/${currentOrderId}/payment`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                paymentCode: paymentCode
            })
        });

        if (!response.ok) {
            const error = await response.text();
            alert('Failed to submit payment: ' + error);
            return;
        }

        alert('Payment submitted. Processing...');
        document.getElementById('payment-code').value = '';

    } catch (error) {
        alert('Error submitting payment: ' + error.message);
    }
}

// Cancel order
async function cancelOrder() {
    if (!confirm('Are you sure you want to cancel this order?')) {
        return;
    }

    try {
        const response = await fetch(`/api/orders/${currentOrderId}`, {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.text();
            alert('Failed to cancel order: ' + error);
            return;
        }

        alert('Order cancelled successfully');

    } catch (error) {
        alert('Error cancelling order: ' + error.message);
    }
}

// Reset booking (make new booking)
function resetBooking() {
    stopStatusPolling();
    currentOrderId = null;
    selectedSeats.clear();
    isUpdatingSeats = false;

    document.getElementById('order-section').style.display = 'none';
    document.getElementById('book-btn').disabled = false;
    document.getElementById('payment-code').value = '';
    document.getElementById('userId').value = '';

    updateSelectedSeatsDisplay();
    htmx.trigger('#seat-grid', 'load');
}

// Stop status polling
function stopStatusPolling() {
    if (statusPollInterval) {
        clearInterval(statusPollInterval);
        statusPollInterval = null;
    }
}

// Enable book button when user ID is entered
document.getElementById('userId').addEventListener('input', function() {
    updateSelectedSeatsDisplay();
});
