package outray

import (
	"encoding/json"
	"testing"
)

// Mock websocket communication using net.Pipe is complex without mocking the library.
// We will test the types and basic struct initialization for now to ensure no compilation errors.

func TestConfig(t *testing.T) {
	c := NewClient(
		WithServerURL("ws://localhost"),
		WithAPIKey("test"),
	)
	if c.config.ServerURL != "ws://localhost" {
		t.Errorf("Expected ServerURL to be set")
	}
}

func TestMessageTypes(t *testing.T) {
	if MsgTypeOpenTunnel != "open_tunnel" {
		t.Error("Wrong message type const")
	}
}

func TestJSONMarshaling(t *testing.T) {
	req := IncomingRequest{
		ID:     "123",
		Method: "GET",
		Path:   "/",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var back IncomingRequest
	json.Unmarshal(data, &back)
	if back.ID != "123" {
		t.Error("JSON roundtrip failed")
	}
}
