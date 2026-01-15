# Outray Go SDK

A minimal, idiomatic Go SDK for Outray, enabling local services to be exposed to the public internet via WebSocket tunnels.

## Features

- **Protocol Agnostic**: Supports HTTP, TCP, and UDP tunneling.
- **Zero Dependencies**: Minimal footprint, depending only on `gorilla/websocket`.
- **Production Ready**: Includes auto-reconnection, context support, and thread-safe operations.

## Installation

```bash
go get github.com/sodiqscript111/outray-go
```

## Usage

### HTTP Tunnel

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/sodiqscript111/outray-go"
)

func main() {
	client := outray.NewClient(
		outray.WithServerURL("wss://api.outray.dev"),
		outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
		outray.WithProtocol("http"),
		outray.WithPort(8080),
		outray.WithOnOpen(func(url string) {
			log.Printf("Tunnel Online: %s", url)
		}),
		outray.WithOnRequest(func(req outray.IncomingRequest) outray.IncomingResponse {
			return outray.IncomingResponse{
				StatusCode: 200,
				Body:       []byte("Hello from Go!"),
			}
		}),
	)

	if err := client.Connect(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

### TCP Tunnel

Exposing a local TCP service (e.g., SSH or PostgreSQL).

```go
client := outray.NewClient(
	outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
	outray.WithProtocol("tcp"),
	outray.WithPort(5432), // Local Postgres port
)

// Blocks until context cancelled
client.Connect(context.Background())
```

### UDP Tunnel

Exposing a local UDP service (e.g., DNS or Game Server).

```go
client := outray.NewClient(
	outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
	outray.WithProtocol("udp"),
	outray.WithPort(53), // Local DNS port
)

client.Connect(context.Background())
```

## Configuration Options

- `WithAPIKey(key string)`: Sets the authentication key.
- `WithProtocol(proto string)`: "http", "tcp", or "udp".
- `WithPort(port int)`: The local port to tunnel to.
- `WithServerURL(url string)`: Overrides the default Outray server URL.
- `WithSubdomain(subdomain string)`: Request a custom subdomain (e.g., "myapp" for myapp.outray.dev).
- `WithLogger(l Logger)`: Sets a custom logger (must implement `Printf`).
- `WithOnOpen(fn func(url string))`: Callback when a tunnel is successfully established.
- `WithOnRequest(fn)`: Handler for incoming HTTP requests.
- `WithOnError(fn)`: Callback for non-fatal errors.
- `WithRequestMiddleware(fn)`: Intercept requests before forwarding to localhost.
- `WithResponseMiddleware(fn)`: Modify responses before sending back.

## Middleware

Middleware allows you to intercept and modify HTTP requests/responses as they pass through the tunnel.

### Request Middleware

Runs before forwarding to your local service. Can modify the request or return an early response.

```go
client := outray.NewClient(
	outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
	outray.WithPort(8080),
	outray.WithRequestMiddleware(func(req *outray.IncomingRequest) *outray.IncomingResponse {
		// Log all requests
		log.Printf("[%s] %s", req.Method, req.Path)

		// Block access to admin routes
		if req.Path == "/admin" {
			return &outray.IncomingResponse{
				StatusCode: 403,
				Body:       []byte("Forbidden"),
			}
		}

		// Add custom header before forwarding
		req.Headers["X-Tunnel"] = "outray"

		return nil // Continue to local service
	}),
)
```

### Response Middleware

Runs after receiving response from your local service. Can modify headers or body.

```go
client := outray.NewClient(
	outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
	outray.WithPort(8080),
	outray.WithResponseMiddleware(func(req *outray.IncomingRequest, resp *outray.IncomingResponse) {
		// Add CORS headers
		resp.Headers["Access-Control-Allow-Origin"] = "*"
	}),
)
```

