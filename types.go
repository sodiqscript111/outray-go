package outray

import "encoding/json"

// MessageType constants
const (
	MsgTypeOpenTunnel    = "open_tunnel"
	MsgTypeTunnelOpened  = "tunnel_opened"
	MsgTypeRequest       = "request"
	MsgTypeResponse      = "response"
	MsgTypeError         = "error"
	MsgTypeTCPConnection = "tcp_connection"
	MsgTypeTCPData       = "tcp_data"
	MsgTypeUDPData       = "udp_data"
	MsgTypeUDPResponse   = "udp_response"
)

// ... (Existing structs)

// TCPConnection represents a request to open a new TCP stream to local.
type TCPConnection struct {
	ID   string `json:"connectionId"`
	Type string `json:"type"` // "tcp_connection"
}

// TCPData represents incoming or outgoing TCP data.
type TCPData struct {
	Type         string `json:"type"` // "tcp_data"
	ConnectionID string `json:"connectionId"`
	Data         string `json:"data"` // Base64 encoded
}

// UDPData represents an incoming UDP packet.
type UDPData struct {
	Type          string `json:"type"` // "udp_data"
	PacketID      string `json:"packetId"`
	Data          string `json:"data"` // Base64 encoded
	SourceAddress string `json:"sourceAddress"`
	SourcePort    int    `json:"sourcePort"`
}

// UDPResponse represents an outgoing UDP response packet.
type UDPResponse struct {
	Type     string `json:"type"` // "udp_response"
	PacketID string `json:"packetId"`
	Data     string `json:"data"` // Base64 encoded
}

// OpenTunnelRequest is the handshake message sent by the client.
type OpenTunnelRequest struct {
	Type     string `json:"type"`
	APIKey   string `json:"apiKey"`
	Protocol string `json:"protocol"` // "http", "tcp", "udp"
	Port     int    `json:"port"`
}

// ServerMessage represents any message received from the server.
// We use a RawMessage for the payload to decode based on Type.
type ServerMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"` // Some messages might flatten this, but let's assume standard handling or handle based on specific message structure if flattened.
	// If the server sends flat JSON, we might need custom unmarshaling or a map.
	// Based on "tunnel_opened", "request", "error", we'll check fields directly if it's flat.
	// Let's assume flat for now as that's common in simple WS protocols.

	// Common fields if flat:
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"` // For errors
}

// IncomingRequest represents a request forwarded from the public tunnel.
type IncomingRequest struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

// IncomingResponse represents a response sent back to the server.
type IncomingResponse struct {
	Type       string            `json:"type"` // "response"
	ID         string            `json:"id"`   // Matches Request ID
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
}

// Map-based helper for generic decoding if needed
type GenericMessage map[string]interface{}
