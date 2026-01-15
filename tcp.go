package outray

import (
	"encoding/base64"
	"fmt"
	"net"
)

func (c *Client) handleTCPConnection(connID string) {
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
