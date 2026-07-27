from flask import Flask, request, jsonify
import uuid
import random
import datetime
import logging
import threading
import traceback

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

orders = {}
request_count = 0


def heartbeat():
    while True:
        logger.info("[order] I am alive schnitzel")
        threading.Event().wait(5)

threading.Thread(target=heartbeat, daemon=True).start()


FAILURE_REASONS = [
    {
        "exception": "InventoryError: SKU schnitzel_classic temporarily unavailable",
        "log": "Order {order_id} failed — could not reserve {quantity}x {item} due to upstream inventory sync lag (warehouse region: eu-central-1, last sync: {ts})",
    },
    {
        "exception": "PaymentGatewayTimeout: gateway did not respond within 3000ms",
        "log": "Payment pre-auth for order {order_id} timed out after 3s — customer may retry, item: {item}, qty: {quantity}",
    },
    {
        "exception": "ValidationError: quantity must be between 1 and 50",
        "log": "Order {order_id} rejected by validation layer — item={item}, requested_quantity={quantity}, max_allowed=50, source_ip={ip}",
    },
    {
        "exception": "KitchenCapacityExceeded: current load 48/50",
        "log": "Kitchen capacity exceeded for order {order_id}, cannot accept {quantity}x {item} right now — current utilization 96%, cool-down ETA ~4min",
    },
    {
        "exception": "DatabaseError: deadlock detected on table 'orders'",
        "log": "Order {order_id} hit a transient DB deadlock while inserting {quantity}x {item} — will not retry automatically, created_at={ts}",
    },
]


@app.route("/orders", methods=["POST"])
def create_order():
    global request_count
    request_count += 1

    data = request.get_json()
    order_id = str(uuid.uuid4())[:8]
    item = data.get("item", "schnitzel")
    quantity = data.get("quantity", 1)
    ts = datetime.datetime.utcnow().isoformat()

    # 1st request: always succeed, 2nd: always fail, 3rd+: ~20% random failure
    should_fail = (request_count == 2) or (request_count > 2 and random.random() < 0.2)
    if should_fail:
        reason = random.choice(FAILURE_REASONS)
        try:
            raise RuntimeError(reason["exception"])
        except RuntimeError:
            logger.error(
                reason["log"].format(
                    order_id=order_id,
                    item=item,
                    quantity=quantity,
                    ts=ts,
                    ip=request.remote_addr,
                ),
                exc_info=True,
            )
            return jsonify({
                "order_id": order_id,
                "status": "failed",
                "error": reason["exception"].split(":")[0],
            }), 500

    order = {
        "order_id": order_id,
        "item": item,
        "quantity": quantity,
        "status": "confirmed",
        "created_at": ts,
    }
    orders[order_id] = order
    print(f"[ORDER] New order {order_id}: {order['quantity']}x {order['item']}")
    return jsonify(order), 201


@app.route("/orders/<order_id>", methods=["GET"])
def get_order(order_id):
    order = orders.get(order_id)
    if not order:
        return jsonify({"error": "Order not found"}), 404
    return jsonify(order)


@app.route("/orders", methods=["GET"])
def list_orders():
    return jsonify(list(orders.values()))


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8081, debug=False, use_reloader=False)
