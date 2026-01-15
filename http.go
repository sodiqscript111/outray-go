package outray

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (c *Client) proxyHTTP(req IncomingRequest) IncomingResponse {
	targetURL := fmt.Sprintf("http://localhost:%d%s", c.config.Port, req.Path)

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

	for k, v := range req.Headers {
		proxyReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return IncomingResponse{StatusCode: 502, Body: []byte(fmt.Sprintf("Proxy Error: %v", err))}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return IncomingResponse{StatusCode: 500, Body: []byte(err.Error())}
	}

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = v[0]
	}

	return IncomingResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       body,
	}
}
