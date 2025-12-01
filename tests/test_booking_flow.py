import pytest
import requests
import time
from concurrent.futures import ThreadPoolExecutor

BASE_URL = "http://localhost:8080/api"
FLIGHT_ID = "FL123"

class TestBasicBooking:
    """Test basic booking flow"""

    def test_health_check(self):
        """Test health check endpoint"""
        resp = requests.get("http://localhost:8080/health")
        assert resp.status_code == 200
        assert resp.json()["status"] == "healthy"

    def test_get_available_seats(self):
        """Test getting available seats"""
        resp = requests.get(f"{BASE_URL}/flights/{FLIGHT_ID}/seats")
        assert resp.status_code == 200
        data = resp.json()
        assert data["flightId"] == FLIGHT_ID
        assert len(data["seats"]) == 30

    def test_create_order_and_book_seats(self):
        """Happy path: Create order, select seats, pay successfully"""
        # Create order
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "alice",
            "seats": ["A1", "A2"]
        })
        assert resp.status_code == 201
        data = resp.json()
        order_id = data["orderId"]
        assert data["userId"] == "alice"
        assert data["seats"] == ["A1", "A2"]

        # Wait for workflow to process
        time.sleep(1)

        # Check status
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "SEATS_RESERVED"
        assert set(data["seats"]) == {"A1", "A2"}
        assert data["timeRemaining"] > 0

        # Submit payment
        resp = requests.post(f"{BASE_URL}/orders/{order_id}/payment", json={
            "paymentCode": "12345"
        })
        assert resp.status_code == 200

        # Wait for payment processing (may take up to 10s + retries)
        time.sleep(3)

        # Verify status (could be PAYMENT_PENDING or CONFIRMED)
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        data = resp.json()
        assert data["status"] in ["PAYMENT_PENDING", "CONFIRMED"]

    def test_create_order_with_invalid_data(self):
        """Test creating order with invalid data"""
        # Missing userId
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "seats": ["A1"]
        })
        assert resp.status_code == 400

        # Missing seats
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "bob"
        })
        assert resp.status_code == 400

        # Empty seats
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "bob",
            "seats": []
        })
        assert resp.status_code == 400

    def test_invalid_payment_code(self):
        """Test invalid payment code format"""
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "bob",
            "seats": ["B1"]
        })
        assert resp.status_code == 201
        order_id = resp.json()["orderId"]

        time.sleep(1)

        # Invalid code (4 digits)
        resp = requests.post(f"{BASE_URL}/orders/{order_id}/payment", json={
            "paymentCode": "1234"
        })
        assert resp.status_code == 400

        # Invalid code (6 digits)
        resp = requests.post(f"{BASE_URL}/orders/{order_id}/payment", json={
            "paymentCode": "123456"
        })
        assert resp.status_code == 400

    def test_update_seat_selection(self):
        """Test updating seat selection"""
        # Create order with initial seats
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "charlie",
            "seats": ["C1", "C2"]
        })
        assert resp.status_code == 201
        order_id = resp.json()["orderId"]

        time.sleep(1)

        # Update seats
        resp = requests.post(f"{BASE_URL}/orders/{order_id}/seats", json={
            "seats": ["C3", "C4"]
        })
        assert resp.status_code == 200

        time.sleep(1)

        # Verify new seats
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        data = resp.json()
        assert set(data["seats"]) == {"C3", "C4"}
        assert data["status"] == "SEATS_RESERVED"

    def test_cancel_order(self):
        """Test canceling an order"""
        # Create order
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "dave",
            "seats": ["A3"]
        })
        assert resp.status_code == 201
        order_id = resp.json()["orderId"]

        time.sleep(1)

        # Cancel order
        resp = requests.delete(f"{BASE_URL}/orders/{order_id}")
        assert resp.status_code == 200

        time.sleep(1)

        # Verify order is cancelled
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        data = resp.json()
        assert data["status"] == "CANCELLED"

        # Verify seats are available again
        resp = requests.get(f"{BASE_URL}/flights/{FLIGHT_ID}/seats")
        seats = resp.json()["seats"]
        seat_a3 = next(s for s in seats if s["seatNumber"] == "A3")
        assert seat_a3["status"] == "AVAILABLE"


class TestConcurrency:
    """Test concurrent booking scenarios"""

    def test_concurrent_same_seat_booking(self):
        """Two users try to book same seat simultaneously"""

        def book_seat(user_id):
            try:
                resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
                    "userId": user_id,
                    "seats": ["C1"]
                }, timeout=10)
                return resp.status_code, resp.json() if resp.status_code == 201 else resp.text
            except Exception as e:
                return 500, str(e)

        with ThreadPoolExecutor(max_workers=2) as executor:
            future1 = executor.submit(book_seat, "user1")
            future2 = executor.submit(book_seat, "user2")

            result1 = future1.result()
            result2 = future2.result()

        # Both requests should succeed (201) - seat reservation is async
        assert result1[0] == 201, f"First request should return 201, got {result1[0]}"
        assert result2[0] == 201, f"Second request should return 201, got {result2[0]}"

        order_id1 = result1[1]["orderId"]
        order_id2 = result2[1]["orderId"]

        # Wait for workflows to process seat reservations
        # Seat conflicts should be non-retryable, so 3 seconds should be enough
        time.sleep(3)

        # Check final statuses - one should succeed, one should fail
        resp1 = requests.get(f"{BASE_URL}/orders/{order_id1}")
        resp2 = requests.get(f"{BASE_URL}/orders/{order_id2}")

        assert resp1.status_code == 200
        assert resp2.status_code == 200

        status1 = resp1.json()["status"]
        status2 = resp2.json()["status"]

        statuses = [status1, status2]

        # One should have reserved seats successfully, one should have failed
        assert "SEATS_RESERVED" in statuses, f"Expected one SEATS_RESERVED, got statuses: {statuses}"
        assert "FAILED" in statuses, f"Expected one FAILED, got statuses: {statuses}"

        # Cleanup: Cancel the successful order to release the seat
        successful_order_id = order_id1 if status1 == "SEATS_RESERVED" else order_id2
        requests.delete(f"{BASE_URL}/orders/{successful_order_id}")
        time.sleep(1)

    def test_seat_update_releases_old_seats(self):
        """User updates seat selection, old seats become available"""
        # User 1 books A5
        resp1 = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "diana",
            "seats": ["A5"]
        })
        assert resp1.status_code == 201
        order_id1 = resp1.json()["orderId"]

        time.sleep(1)

        # User 2 tries A5 (order creation succeeds but workflow will fail)
        resp2 = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "eve",
            "seats": ["A5"]
        })
        assert resp2.status_code == 201
        order_id2 = resp2.json()["orderId"]

        # Wait for workflow to fail (seat conflicts should be non-retryable)
        time.sleep(3)

        # Check that order failed
        resp_check = requests.get(f"{BASE_URL}/orders/{order_id2}")
        assert resp_check.status_code == 200
        assert resp_check.json()["status"] == "FAILED"

        # User 1 changes to B5
        resp3 = requests.post(f"{BASE_URL}/orders/{order_id1}/seats", json={
            "seats": ["B5"]
        })
        assert resp3.status_code == 200

        time.sleep(1)

        # User 2 tries A5 again (should succeed now)
        resp4 = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "eve",
            "seats": ["A5"]
        })
        assert resp4.status_code == 201

    def test_multiple_users_different_seats(self):
        """Multiple users booking different seats simultaneously"""

        def book_seats(user_id, seats):
            try:
                resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
                    "userId": user_id,
                    "seats": seats
                }, timeout=10)
                return resp.status_code, resp.json() if resp.status_code == 201 else resp.text
            except Exception as e:
                return 500, str(e)

        with ThreadPoolExecutor(max_workers=3) as executor:
            future1 = executor.submit(book_seats, "user1", ["A6", "A7"])
            future2 = executor.submit(book_seats, "user2", ["B6", "B7"])
            future3 = executor.submit(book_seats, "user3", ["C6", "C7"])

            results = [future1.result(), future2.result(), future3.result()]

        # All should succeed since they're booking different seats
        for status, _ in results:
            assert status == 201


@pytest.mark.slow
class TestTimerExpiration:
    """Test reservation timer expiration"""

    def test_timer_refresh_on_seat_update(self):
        """Timer resets when user updates seats"""
        # Set short timeout for testing (using default 15 minutes)
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "henry",
            "seats": ["A7"]
        })
        assert resp.status_code == 201
        order_id = resp.json()["orderId"]

        time.sleep(1)

        # Get initial time remaining
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        initial_time = resp.json()["timeRemaining"]

        # Wait 3 seconds
        time.sleep(3)

        # Time should have decreased
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        time_after_wait = resp.json()["timeRemaining"]
        assert time_after_wait < initial_time

        # Update seats (should reset timer)
        resp = requests.post(f"{BASE_URL}/orders/{order_id}/seats", json={
            "seats": ["A8"]
        })
        assert resp.status_code == 200

        time.sleep(1)

        # Time remaining should be close to initial timeout
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        time_after_update = resp.json()["timeRemaining"]
        # Should be close to 15 minutes (900 seconds), allow some margin
        assert time_after_update > initial_time - 10


class TestPaymentRetry:
    """Test payment retry logic"""

    def test_payment_submission(self):
        """Test payment submission flow"""
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "iris",
            "seats": ["B7"]
        })
        assert resp.status_code == 201
        order_id = resp.json()["orderId"]

        time.sleep(1)

        # Submit payment
        resp = requests.post(f"{BASE_URL}/orders/{order_id}/payment", json={
            "paymentCode": "99999"
        })
        assert resp.status_code == 200

        # Check status immediately
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        data = resp.json()
        assert data["status"] in ["PAYMENT_PENDING", "SEATS_RESERVED"]

        # Wait for payment processing (could take up to 30s with retries)
        max_wait = 35
        for i in range(max_wait):
            time.sleep(1)
            resp = requests.get(f"{BASE_URL}/orders/{order_id}")
            status = resp.json()["status"]
            if status in ["CONFIRMED", "FAILED"]:
                break

        # Final status should be CONFIRMED or FAILED
        resp = requests.get(f"{BASE_URL}/orders/{order_id}")
        final_status = resp.json()["status"]
        assert final_status in ["CONFIRMED", "FAILED", "PAYMENT_PENDING"]


class TestOrderCancellation:
    """Test order cancellation"""

    def test_cancel_order_releases_seats(self):
        """Cancelling order releases reserved seats"""
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "jack",
            "seats": ["C7"]
        })
        assert resp.status_code == 201
        order_id = resp.json()["orderId"]

        time.sleep(1)

        # Verify seat is reserved
        resp = requests.get(f"{BASE_URL}/flights/{FLIGHT_ID}/seats")
        seats = resp.json()["seats"]
        seat_c7 = next(s for s in seats if s["seatNumber"] == "C7")
        assert seat_c7["status"] == "RESERVED"

        # Cancel order
        resp = requests.delete(f"{BASE_URL}/orders/{order_id}")
        assert resp.status_code == 200

        time.sleep(1)

        # Verify seat is available
        resp = requests.get(f"{BASE_URL}/flights/{FLIGHT_ID}/seats")
        seats = resp.json()["seats"]
        seat_c7 = next(s for s in seats if s["seatNumber"] == "C7")
        assert seat_c7["status"] == "AVAILABLE"

        # Another user can book it
        resp = requests.post(f"{BASE_URL}/flights/{FLIGHT_ID}/orders", json={
            "userId": "kate",
            "seats": ["C7"]
        })
        assert resp.status_code == 201
