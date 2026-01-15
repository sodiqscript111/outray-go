package outray

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// proxyHTTP proxies an incoming HTTP request to the local service.
func (c *Client) proxyHTTP(req IncomingRequest) IncomingResponse {
	targetURL := fmt.Sprintf("http://localhost:%d%s", c.config.Port, req.Path)

	// Create request body reader
	var bodyReader *strings.Reader
	if len(req.Body) > 0 {
		bodyReader = strings.NewReader(string(req.Body))
	} else {
		bodyReader = strings.NewReader("")
	}

	proxyReq, err := http.NewRequest(req.Method, targetURL, bodyReader)
	if err != nil {
		return IncomingResponse{StatusCode: 500, Body: []byte(err.Error())}
	}

	// Copy headers
	for k, v := range req.Headers {
		proxyReq.Header.Set(k, v)
	}

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return IncomingResponse{StatusCode: 502, Body: []byte(fmt.Sprintf("Proxy Error: %v", err))}
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return IncomingResponse{StatusCode: 500, Body: []byte(err.Error())}
	}

	// Copy response headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = v[0] // Simplify: take first value
	}

	return IncomingResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       body,
	}
}
