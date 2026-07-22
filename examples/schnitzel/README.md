# Schnitzel 🥩

Four-service Python application for ordering Wiener Schnitzel.

## Architecture

| Service        | Port | Description                          |
|----------------|------|--------------------------------------|
| Frontend       | 8080 | Web UI with order button             |
| Order          | 8081 | Creates and stores orders            |
| Delivery       | 8082 | Assigns delivery with random ETA     |
| Loadgenerator  | —    | Sends automated order batches        |

**Flow:** Frontend → Order Service → Delivery Service

## Setup

```bash
pip install -r requirements.txt
```

## Run

Start all services in separate terminals:

```bash
# Terminal 1 – Order Service
python order/app.py

# Terminal 2 – Delivery Service
python delivery/app.py

# Terminal 3 – Frontend
python frontend/app.py

# Terminal 4 – Load Generator (optional)
python loadgenerator/app.py
```

Then open <http://localhost:8080> and click the button to order a schnitzel.
