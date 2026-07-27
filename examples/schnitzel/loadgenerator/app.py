import requests
import random
import time
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

FRONTEND_URL = "http://localhost:8080"


def place_order():
    try:
        resp = requests.post(f"{FRONTEND_URL}/order", timeout=10)
        data = resp.json()
        if data.get("status") == "success":
            order_id = data.get("order", {}).get("order_id", "?")
            eta = data.get("delivery", {}).get("eta", "?")
            logger.info("[loadgenerator] Order %s placed successfully — ETA %s", order_id, eta)
        else:
            error = data.get("order", {}).get("error", "unknown")
            logger.warning("[loadgenerator] Order failed: %s", error)
    except requests.exceptions.RequestException as e:
        logger.error("[loadgenerator] Request error: %s", e)


RUNTIME_SECONDS = 5 * 60

if __name__ == "__main__":
    logger.info("[loadgenerator] Starting — sending 1-10 orders every 5-10 seconds for %d minutes", RUNTIME_SECONDS // 60)
    start_time = time.time()
    while time.time() - start_time < RUNTIME_SECONDS:
        batch_size = random.randint(1, 10)
        logger.info("[loadgenerator] Sending batch of %d order(s)", batch_size)
        for _ in range(batch_size):
            place_order()
        time.sleep(random.uniform(5, 10))
    logger.info("[loadgenerator] Runtime of %d minutes reached — shutting down", RUNTIME_SECONDS // 60)
