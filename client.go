package outray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Logger interface {
	Printf(format string, v ...interface{})
}

type Option func(*Client)

func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.config.APIKey = key
	}
}

func WithProtocol(p string) Option {
	return func(c *Client) {
		c.config.Protocol = p
	}
}

func WithPort(p int) Option {
	return func(c *Client) {
		c.config.Port = p
	}
}

func WithServerURL(url string) Option {
	return func(c *Client) {
		c.config.ServerURL = url
	}
}

func WithLogger(l Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

func WithOnOpen(fn func(url string)) Option {
	return func(c *Client) {
		c.config.OnOpen = fn
	}
}

func WithOnRequest(fn func(req IncomingRequest) IncomingResponse) Option {
	return func(c *Client) {
		c.config.OnRequest = fn
	}
}

func WithOnError(fn func(err error)) Option {
	return func(c *Client) {
		c.config.OnError = fn
	}
}

func WithSubdomain(subdomain string) Option {
	return func(c *Client) {
		c.config.Subdomain = subdomain
	}
}

type Config struct {
	ServerURL string
	APIKey    string
	Protocol  string
	Port      int
	Subdomain string
	OnOpen    func(url string)
	OnRequest func(req IncomingRequest) IncomingResponse
	OnError   func(err error)
}

type Client struct {
	config Config
	conn   *websocket.Conn
	mu     sync.Mutex
	closed bool
	logger Logger

	tcpConns   map[string]net.Conn
	tcpConnsMu sync.Mutex
}

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
			backoff = time.Second
		}
	}
}

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

	const (
		pingPeriod = 9 * time.Second
	)

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.mu.Lock()
				if c.closed {
					c.mu.Unlock()
					return
				}
				if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
					c.mu.Unlock()
					c.logf("Ping failed: %v", err)
					return
				}
				c.mu.Unlock()
			}
		}
	}()

	handshake := OpenTunnelRequest{
		Type:      MsgTypeOpenTunnel,
		APIKey:    c.config.APIKey,
		Protocol:  c.config.Protocol,
		Port:      c.config.Port,
		Subdomain: c.config.Subdomain,
	}

	if err := c.conn.WriteJSON(handshake); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	return c.readLoop()
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	c.tcpConnsMu.Lock()
	for _, conn := range c.tcpConns {
		conn.Close()
	}
	c.tcpConns = make(map[string]net.Conn)
	c.tcpConnsMu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

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
					go func() {
						resp := c.proxyHTTP(req)
						resp.ID = req.ID
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

func (c *Client) logf(format string, v ...interface{}) {
	if c.logger != nil {
		c.logger.Printf(format, v...)
	}
}
