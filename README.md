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

## Protocol Overview

| Protocol | Remote Port | Local Port | Public Access |
|----------|-------------|------------|---------------|
| HTTP | Not required | `WithPort()` | `subdomain.outray.app` |
| TCP | `WithRemotePort()` (20000-30000) | `WithPort()` | `host.outray.app:port` |
| UDP | `WithRemotePort()` (30001-40000) | `WithPort()` | `host.outray.app:port` |

## Usage

### HTTP Tunnel

HTTP tunnels use subdomain-based routing, so you only need to specify the local port.

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
		outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
		outray.WithProtocol("http"),
		outray.WithPort(8080),
		outray.WithOnOpen(func(url string) {
			log.Printf("Tunnel Online: %s", url)
		}),
	)

	if err := client.Connect(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

### TCP Tunnel

TCP tunnels require a remote port in the range **20000-30000**.

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
		outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
		outray.WithProtocol("tcp"),
		outray.WithRemotePort(25000),
		outray.WithPort(5432),
		outray.WithOnOpen(func(url string) {
			log.Printf("TCP Tunnel Online: %s", url)
		}),
	)

	if err := client.Connect(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

### UDP Tunnel

UDP tunnels require a remote port in the range **30001-40000**.

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
		outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
		outray.WithProtocol("udp"),
		outray.WithRemotePort(35000),
		outray.WithPort(53),
		outray.WithOnOpen(func(url string) {
			log.Printf("UDP Tunnel Online: %s", url)
		}),
	)

	if err := client.Connect(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

## Configuration Options

| Option | Description |
|--------|-------------|
| `WithAPIKey(key string)` | Sets the authentication key |
| `WithProtocol(proto string)` | "http", "tcp", or "udp" |
| `WithPort(port int)` | Local port to forward traffic to |
| `WithRemotePort(port int)` | Server-side port (TCP: 20000-30000, UDP: 30001-40000) |
| `WithServerURL(url string)` | Overrides the default Outray server URL |
| `WithSubdomain(subdomain string)` | Request a custom subdomain |
| `WithCustomDomain(domain string)` | Use a custom domain |
| `WithForceTakeover(bool)` | Force takeover of existing tunnel |
| `WithLogger(l Logger)` | Sets a custom logger (must implement `Printf`) |
| `WithOnOpen(fn func(url string))` | Callback when tunnel is established |
| `WithOnRequest(fn)` | Handler for incoming HTTP requests |
| `WithOnError(fn)` | Callback for non-fatal errors |
| `WithRequestMiddleware(fn)` | Intercept requests before forwarding |
| `WithResponseMiddleware(fn)` | Modify responses before sending back |

## Middleware

Middleware allows you to intercept and modify HTTP requests/responses as they pass through the tunnel.

### Request Middleware

Runs before forwarding to your local service. Can modify the request or return an early response.

```go
client := outray.NewClient(
	outray.WithAPIKey(os.Getenv("OUTRAY_API_KEY")),
	outray.WithPort(8080),
	outray.WithRequestMiddleware(func(req *outray.IncomingRequest) *outray.IncomingResponse {
		log.Printf("[%s] %s", req.Method, req.Path)

		if req.Path == "/admin" {
			return &outray.IncomingResponse{
				StatusCode: 403,
				Body:       []byte("Forbidden"),
			}
		}

		req.Headers["X-Tunnel"] = "outray"
		return nil
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
		resp.Headers["Access-Control-Allow-Origin"] = "*"
	}),
)
```
