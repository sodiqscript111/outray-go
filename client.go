package outray

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Logger is a simple interface for logging.
type Logger interface {
	Printf(format string, v ...interface{})
}

// Option configures the Client.
type Option func(*Client)

// WithAPIKey sets the API Key.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.config.APIKey = key
	}
}

// WithProtocol sets the protocol ("http", "tcp", "udp").
func WithProtocol(p string) Option {
	return func(c *Client) {
		c.config.Protocol = p
	}
}

// WithPort sets the local port to tunnel.
func WithPort(p int) Option {
	return func(c *Client) {
		c.config.Port = p
	}
}

// WithServerURL sets the Outray server URL.
func WithServerURL(url string) Option {
	return func(c *Client) {
		c.config.ServerURL = url
	}
}

// WithLogger sets the logger.
func WithLogger(l Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

// WithOnOpen sets the callback for when the tunnel opens.
func WithOnOpen(fn func(url string)) Option {
	return func(c *Client) {
		c.config.OnOpen = fn
	}
}

// WithOnRequest sets the callback for incoming HTTP requests.
func WithOnRequest(fn func(req IncomingRequest) IncomingResponse) Option {
	return func(c *Client) {
		c.config.OnRequest = fn
	}
}

// WithOnError sets the callback for errors.
func WithOnError(fn func(err error)) Option {
	return func(c *Client) {
		c.config.OnError = fn
	}
}

// Config holds the configuration (internal use mostly now).
type Config struct {
	ServerURL string
	APIKey    string
	Protocol  string
	Port      int
	OnOpen    func(url string)
	OnRequest func(req IncomingRequest) IncomingResponse
	OnError   func(err error)
}

// Client is the Outray SDK client.
type Client struct {
	config Config
	conn   *websocket.Conn
	mu     sync.Mutex
	closed bool
	logger Logger

	tcpConns   map[string]net.Conn
	tcpConnsMu sync.Mutex
}

// NewClient creates a new Outray client with options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		config: Config{
			ServerURL: "wss://api.outray.dev",
			Protocol:  "http",
		},
		tcpConns: make(map[string]net.Conn),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Connect establishes the WebSocket connection and maintains it (auto-reconnects).
// It blocks until the context is done.
func (c *Client) Connect(ctx context.Context) error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.connectOnce(ctx); err != nil {
			c.logf("Connection error: %v. Retrying in %v...", err, backoff)
			if c.config.OnError != nil {
				c.safeOnError(err)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		} else {
			// If connectOnce returns nil (meaning unintentional disconnect/close), reset backoff
			backoff = time.Second
		}
	}
}

// connectOnce attempts a single connection and blocks on readLoop.
func (c *Client) connectOnce(ctx context.Context) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, c.config.ServerURL, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.closed = false
	c.mu.Unlock()

	defer c.Close()

	// Handshake
	handshake := OpenTunnelRequest{
		Type:     MsgTypeOpenTunnel,
		APIKey:   c.config.APIKey,
		Protocol: c.config.Protocol,
		Port:     c.config.Port,
	}

	if err := c.conn.WriteJSON(handshake); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	// This blocks until connection is lost
	return c.readLoop()
}

// Close closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	// Close all TCP connections
	c.tcpConnsMu.Lock()
	for _, conn := range c.tcpConns {
		conn.Close()
	}
	c.tcpConns = make(map[string]net.Conn) // Reset map
	c.tcpConnsMu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendResponse sends a response message back to the server.
func (c *Client) SendResponse(resp IncomingResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("client is closed")
	}
	resp.Type = MsgTypeResponse
	return c.conn.WriteJSON(resp)
}

func (c *Client) safeCallback(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			c.logf("Panic in callback: %v", r)
		}
	}()
	fn()
}

func (c *Client) safeOnError(err error) {
	c.safeCallback(func() { c.config.OnError(err) })
}

func (c *Client) handleTCPConnection(connID string) {
	// Dial local service
	target := fmt.Sprintf("localhost:%d", c.config.Port)
	localConn, err := net.Dial("tcp", target)
	if err != nil {
		if c.config.OnError != nil {
			c.safeOnError(fmt.Errorf("failed to dial local tcp %s: %w", target, err))
		}
		return
	}

	c.tcpConnsMu.Lock()
	c.tcpConns[connID] = localConn
	c.tcpConnsMu.Unlock()

	// Start reading from local conn and sending to server
	go func() {
		defer func() {
			localConn.Close()
			c.tcpConnsMu.Lock()
			delete(c.tcpConns, connID)
			c.tcpConnsMu.Unlock()
		}()

		buf := make([]byte, 4096)
		for {
			n, err := localConn.Read(buf)
			if err != nil {
				return
			}

			data := base64.StdEncoding.EncodeToString(buf[:n])
			msg := TCPData{
				Type:         MsgTypeTCPData,
				ConnectionID: connID,
				Data:         data,
			}

			c.mu.Lock()
			if !c.closed {
				c.conn.WriteJSON(msg)
			}
			c.mu.Unlock()
		}
	}()
}

func (c *Client) handleTCPData(connID string, dataB64 string) {
	c.tcpConnsMu.Lock()
	localConn, ok := c.tcpConns[connID]
	c.tcpConnsMu.Unlock()

	if !ok {
		return
	}

	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return
	}

	localConn.Write(data)
}

func (c *Client) handleUDPData(packet UDPData) {
	target := fmt.Sprintf("localhost:%d", c.config.Port)
	conn, err := net.Dial("udp", target)
	if err != nil {
		if c.config.OnError != nil {
			c.safeOnError(fmt.Errorf("failed to dial local udp %s: %w", target, err))
		}
		return
	}
	defer conn.Close()

	data, err := base64.StdEncoding.DecodeString(packet.Data)
	if err != nil {
		return
	}

	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(data); err != nil {
		return
	}

	respBuf := make([]byte, 4096)
	n, err := conn.Read(respBuf)
	if err != nil {
		return
	}

	respData := base64.StdEncoding.EncodeToString(respBuf[:n])
	respMsg := UDPResponse{
		Type:     MsgTypeUDPResponse,
		PacketID: packet.PacketID,
		Data:     respData,
	}

	c.mu.Lock()
	if !c.closed {
		c.conn.WriteJSON(respMsg)
	}
	c.mu.Unlock()
}

func (c *Client) readLoop() error {
	for {
		var raw map[string]interface{}
		if err := c.conn.ReadJSON(&raw); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return nil // Clean exit
			}
			return err
		}

		msgType, ok := raw["type"].(string)
		if !ok {
			continue
		}

		switch msgType {
		case MsgTypeTunnelOpened:
			if c.config.OnOpen != nil {
				url, _ := raw["url"].(string)
				c.safeCallback(func() { c.config.OnOpen(url) })
			}
		case MsgTypeTCPConnection:
			connID, _ := raw["connectionId"].(string)
			go c.handleTCPConnection(connID)
		case MsgTypeTCPData:
			connID, _ := raw["connectionId"].(string)
			data, _ := raw["data"].(string)
			c.handleTCPData(connID, data)
		case MsgTypeUDPData:
			var packet UDPData
			bytes, _ := json.Marshal(raw)
			if err := json.Unmarshal(bytes, &packet); err == nil {
				go c.handleUDPData(packet)
			}
		case MsgTypeRequest:
			data, _ := json.Marshal(raw)
			var req IncomingRequest
			if err := json.Unmarshal(data, &req); err == nil {
				if c.config.OnRequest != nil {
					c.safeCallback(func() {
						resp := c.config.OnRequest(req)
						resp.ID = req.ID
						if err := c.SendResponse(resp); err != nil {
							if c.config.OnError != nil {
								c.safeOnError(fmt.Errorf("send response error: %w", err))
							}
						}
					})
				} else if c.config.Port > 0 && c.config.Protocol == "http" {
					// Default behavior: Proxy to localhost
					go func() {
						resp := c.proxyHTTP(req)
						resp.ID = req.ID // Ensure ID is set
						if err := c.SendResponse(resp); err != nil {
							if c.config.OnError != nil {
								c.safeOnError(fmt.Errorf("proxy send response error: %w", err))
							}
						}
					}()
				}
			}
		case MsgTypeError:
			if c.config.OnError != nil {
				msg, _ := raw["message"].(string)
				c.safeOnError(errors.New(msg))
			}
		}
	}
}

func (c *Client) proxyHTTP(req IncomingRequest) IncomingResponse {
	targetURL := fmt.Sprintf("http://localhost:%d%s", c.config.Port, req.Path)

	// Create request
	// Note: We might need to handle body more efficiently in future (io.Reader)
	// but for now, bytes is fine.
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

	// Execute
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return IncomingResponse{StatusCode: 502, Body: []byte(fmt.Sprintf("Proxy Error: %v", err))}
	}
	defer resp.Body.Close()

	// Read response
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

func (c *Client) logf(format string, v ...interface{}) {
	if c.logger != nil {
		c.logger.Printf(format, v...)
	}
}
