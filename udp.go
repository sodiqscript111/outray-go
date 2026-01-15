package outray

import (
	"encoding/base64"
	"fmt"
	"net"
	"time"
)

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
