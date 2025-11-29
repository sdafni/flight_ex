import pytest
import requests
import time

BASE_URL = "http://localhost:8080"

@pytest.fixture(scope="session", autouse=True)
def wait_for_services():
    """Wait for services to be ready before running tests"""
    max_retries = 60
    for i in range(max_retries):
        try:
            resp = requests.get(f"{BASE_URL}/health", timeout=5)
            if resp.status_code == 200:
                print("\nServices are ready!")
                return
        except requests.ConnectionError:
            pass
        except requests.exceptions.Timeout:
            pass

        if i % 10 == 0:
            print(f"\nWaiting for services to start... ({i}/{max_retries})")
        time.sleep(1)

    pytest.fail("Services did not start in time")

@pytest.fixture(scope="function", autouse=True)
def reset_flight_seats():
    """Reset flight seats before each test"""
    try:
        requests.post(f"{BASE_URL}/api/admin/flights/FL123/reset", timeout=5)
        time.sleep(0.5)  # Give time for reset to complete
    except Exception as e:
        print(f"Warning: Failed to reset flight seats: {e}")
    yield
