from flask import Flask, request, jsonify
import random
import uuid
import logging
import threading

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

deliveries = {}


def heartbeat():
    while True:
        logger.info("[delivery] I am alive schnitzel")
        threading.Event().wait(5)

threading.Thread(target=heartbeat, daemon=True).start()


@app.route("/deliveries", methods=["POST"])
def create_delivery():
    data = request.get_json()
    delivery_id = str(uuid.uuid4())[:8]
    eta_minutes = random.randint(15, 45)
    delivery = {
        "delivery_id": delivery_id,
        "order_id": data.get("order_id"),
        "item": data.get("item", "schnitzel"),
        "status": "dispatched",
        "eta": f"{eta_minutes} minutes",
    }
    deliveries[delivery_id] = delivery
    print(f"[DELIVERY] Delivery {delivery_id} for order {delivery['order_id']} — ETA {eta_minutes}min")
    return jsonify(delivery), 201


@app.route("/deliveries/<delivery_id>", methods=["GET"])
def get_delivery(delivery_id):
    delivery = deliveries.get(delivery_id)
    if not delivery:
        return jsonify({"error": "Delivery not found"}), 404
    return jsonify(delivery)


@app.route("/deliveries", methods=["GET"])
def list_deliveries():
    return jsonify(list(deliveries.values()))


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8082, debug=False, use_reloader=False)
