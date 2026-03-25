# Go Round-Robin Load Balancer

A simple round-robin load balancer written in Go. It distributes incoming HTTP requests across multiple backend servers, performs periodic health checks, and supports dynamic addition/removal of servers at runtime.

## Features

- **Round-Robin Scheduling** — Cycles through backend servers evenly, skipping any that are marked unhealthy.
- **Health Checks** — Pings each backend's `/health` endpoint every 10 seconds to detect failures and recoveries automatically.
- **Dynamic Server Updates** — Add or remove backends on the fly via a `POST /update-servers` API without restarting the load balancer.
- **Reverse Proxy** — Uses Go's `httputil.ReverseProxy` to forward requests transparently.

## Project Structure

```
.
├── main.go        # Load balancer (round-robin + health checks + reverse proxy)
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

### 1. Start 4 Backend Servers

Open **4 separate terminals** and start a backend on each port:

```bash
# Terminal 1
./backend -port 8001

# Terminal 2
./backend -port 8002

# Terminal 3
./backend -port 8003

# Terminal 4
./backend -port 8004
```

### 2. Start the Load Balancer

In a **5th terminal**, start the load balancer:

```bash
./loadbalancer -port 8080
```

The load balancer will perform an initial health check and begin routing traffic across all 4 backends.

### 3. Send Requests

From any terminal, send a few requests and observe round-robin distribution:

```bash
curl http://localhost:8080
curl http://localhost:8080
curl http://localhost:8080
curl http://localhost:8080
```

You should see responses cycling through the backends:

```
Hello from server on port 8001!
Hello from server on port 8002!
Hello from server on port 8003!
Hello from server on port 8004!
```

### 4. Simulate a Server Going Down

Close one of the backend terminals (e.g. the one running on port `8003`) by pressing `Ctrl+C`. Now send more requests:

```bash
curl http://localhost:8080
curl http://localhost:8080
curl http://localhost:8080
```

Within 10 seconds (the health check interval), the load balancer detects that `8003` is down and stops routing traffic to it. Requests are distributed only among the remaining healthy backends.

### 5. Bring the Server Back Up

Restart the backend on port `8003`:

```bash
./backend -port 8003
```

The next health check will detect that `8003` is alive again and automatically add it back into the rotation — no restart needed.

### 6. Dynamically Update Server List

You can also change the entire set of backends at runtime using the `/update-servers` endpoint:

```bash
curl -X POST http://localhost:8080/update-servers \
  -H "Content-Type: application/json" \
  -d '{"server_urls": ["http://localhost:8001", "http://localhost:8002"]}'
```

This replaces the backend list with only ports `8001` and `8002`. To add them all back:

```bash
curl -X POST http://localhost:8080/update-servers \
  -H "Content-Type: application/json" \
  -d '{"server_urls": ["http://localhost:8001", "http://localhost:8002", "http://localhost:8003", "http://localhost:8004"]}'
```

## How It Works

1. **Startup** — The load balancer initializes reverse proxies for each backend URL and runs an initial health check.
2. **Request Handling** — On each incoming request, an atomic counter selects the next server index. If that server is unhealthy, it skips ahead to the next alive server.
3. **Health Checks** — A background goroutine hits `/health` on every backend every 10 seconds, updating each server's alive status.
4. **Dynamic Updates** — `POST /update-servers` accepts a JSON body with a new list of server URLs, rebuilds the server pool, and triggers an immediate health check.
