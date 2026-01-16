package outray

import "encoding/json"

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

type TCPConnection struct {
	ID   string `json:"connectionId"`
	Type string `json:"type"`
}

type TCPData struct {
	Type         string `json:"type"`
	ConnectionID string `json:"connectionId"`
	Data         string `json:"data"`
}

type UDPData struct {
	Type          string `json:"type"`
	PacketID      string `json:"packetId"`
	Data          string `json:"data"`
	SourceAddress string `json:"sourceAddress"`
	SourcePort    int    `json:"sourcePort"`
}

type UDPResponse struct {
	Type     string `json:"type"`
	PacketID string `json:"packetId"`
	Data     string `json:"data"`
}

type OpenTunnelRequest struct {
	Type          string `json:"type"`
	APIKey        string `json:"apiKey,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	Port          int    `json:"remotePort,omitempty"`
	Subdomain     string `json:"subdomain,omitempty"`
	CustomDomain  string `json:"customDomain,omitempty"`
	ForceTakeover bool   `json:"forceTakeover,omitempty"`
}

type ServerMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	ID      string          `json:"id,omitempty"`
	Message string          `json:"message,omitempty"`
}

type IncomingRequest struct {
	ID      string            `json:"requestId"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

type IncomingResponse struct {
	Type       string            `json:"type"`
	ID         string            `json:"requestId"`
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
}

type GenericMessage map[string]interface{}
