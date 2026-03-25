# Go Weighted Round-Robin Load Balancer

A weighted round-robin load balancer written in Go. It distributes incoming HTTP requests across multiple backend servers proportionally to their assigned weights, performs periodic health checks, and supports dynamic reconfiguration at runtime.

## Features

- **Weighted Round-Robin Scheduling** — Routes traffic proportionally based on server weights using a smooth weighted round-robin algorithm, ensuring even distribution without long bursts to a single backend.
- **Health Checks** — Pings each backend's `/health` endpoint every 10 seconds to detect failures and recoveries automatically. Unhealthy servers are skipped during selection.
- **Dynamic Server Updates** — Add, remove, or re-weight backends on the fly via a `POST /update-servers` API without restarting the load balancer.
- **Reverse Proxy** — Uses Go's `httputil.ReverseProxy` to forward requests transparently with automatic error handling.

## Project Structure

```
.
├── main.go        # Load balancer (weighted round-robin + health checks + reverse proxy)
├── backend.go     # Simple backend server (responds with its port number)
└── README.md
```

## Prerequisites

- [Go](https://go.dev/dl/) 1.18 or later

## Build

Build both the backend server and the load balancer:

```bash
go build -o backend backend.go
go build -o loadbalancer main.go
```

## Testing It Out

### 1. Start Backend Servers

Open **3 separate terminals** and start a backend on each port:

```bash
# Terminal 1
./backend -port 8001

# Terminal 2
./backend -port 8002

# Terminal 3
./backend -port 8003
```

### 2. Start the Load Balancer

In another terminal, start the load balancer:

```bash
./loadbalancer -port 8080
```

The load balancer starts with the following default weights:

| Backend | Weight |
|---------|--------|
| `localhost:8001` | 1 |
| `localhost:8002` | 2 |
| `localhost:8003` | 3 |

This means for every 6 requests, `8003` handles ~3, `8002` handles ~2, and `8001` handles ~1.

### 3. Send Requests

Send several requests and observe the weighted distribution:

```bash
for i in $(seq 1 6); do curl -s http://localhost:8080; echo; done
```

Example output (order may vary due to smooth distribution):

```
Hello from server on port 8003!
Hello from server on port 8002!
Hello from server on port 8003!
Hello from server on port 8001!
Hello from server on port 8003!
Hello from server on port 8002!
```

Notice that `8003` (weight 3) receives the most requests, followed by `8002` (weight 2), then `8001` (weight 1).

### 4. Simulate a Server Going Down

Close one of the backend terminals (e.g. `8003`) by pressing `Ctrl+C`. Within 10 seconds (the health check interval), the load balancer detects that `8003` is down and redistributes traffic among the remaining healthy backends proportionally to their weights.

### 5. Bring the Server Back Up

Restart the backend on port `8003`:

```bash
./backend -port 8003
```

The next health check will detect that `8003` is alive again and automatically add it back into the weighted rotation — no restart needed.

### 6. Dynamically Update Server List

You can change the set of backends and their weights at runtime using the `/update-servers` endpoint. The payload is a JSON object mapping server URLs to their weights:

```bash
curl -X POST http://localhost:8080/update-servers \
  -H "Content-Type: application/json" \
  -d '{"server_urls": {"http://localhost:8001": 5, "http://localhost:8002": 3}}'
```

This replaces the backend pool with two servers where `8001` now has a weight of 5 and `8002` has a weight of 3. To restore the original configuration:

```bash
curl -X POST http://localhost:8080/update-servers \
  -H "Content-Type: application/json" \
  -d '{"server_urls": {"http://localhost:8001": 1, "http://localhost:8002": 2, "http://localhost:8003": 3}}'
```

## How It Works

1. **Startup** — The load balancer initializes reverse proxies for each backend URL with their configured weights and runs an initial health check.
2. **Request Handling** — On each incoming request, the smooth weighted round-robin algorithm selects the next server:
   - Each alive server's `CurrentWeight` is incremented by its `Weight`.
   - The server with the highest `CurrentWeight` is selected.
   - The selected server's `CurrentWeight` is decremented by the total weight of all alive servers.
   - This produces a smooth, interleaved distribution (e.g., weights 1:2:3 yield the sequence `C B C A C B` rather than `A B B C C C`).
3. **Health Checks** — A background goroutine hits `/health` on every backend every 10 seconds, updating each server's alive status. Unhealthy servers are excluded from weight calculations.
4. **Dynamic Updates** — `POST /update-servers` accepts a JSON body mapping server URLs to integer weights, rebuilds the server pool, and triggers an immediate health check.
