from flask import Flask, render_template, jsonify
import requests
import os
import logging
import threading

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)


def heartbeat():
    while True:
        logger.info("[frontend] I am alive schnitzel")
        threading.Event().wait(5)

threading.Thread(target=heartbeat, daemon=True).start()

ORDER_SERVICE_URL = os.getenv("ORDER_SERVICE_URL", "http://localhost:8081")
DELIVERY_SERVICE_URL = os.getenv("DELIVERY_SERVICE_URL", "http://localhost:8082")


@app.route("/")
def index():
    return render_template("index.html")


@app.route("/order", methods=["POST"])
def place_order():
    try:
        # Call order service
        order_resp = requests.post(
            f"{ORDER_SERVICE_URL}/orders",
            json={"item": "schnitzel", "quantity": 1},
            timeout=5,
        )
        order_data = order_resp.json()
        order_id = order_data.get("order_id")

        if order_resp.status_code != 201:
            logger.warning("Order %s was not confirmed: %s", order_id, order_data.get("error", "unknown"))
            return jsonify({"status": "failed", "order": order_data}), 500

        # Call delivery service
        delivery_resp = requests.post(
            f"{DELIVERY_SERVICE_URL}/deliveries",
            json={"order_id": order_id, "item": "schnitzel"},
            timeout=5,
        )
        delivery_data = delivery_resp.json()

        logger.info(
            "Schnitzel ordered! Order #%s, estimated delivery: %s",
            order_id,
            delivery_data.get("eta", "unknown"),
        )

        return jsonify(
            {
                "status": "success",
                "order": order_data,
                "delivery": delivery_data,
            }
        )
    except requests.exceptions.RequestException as e:
        return jsonify({"status": "error", "message": str(e)}), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080, debug=False, use_reloader=False)
