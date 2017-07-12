package common

import "testing"

// TestPickRandomPort tests that gets a right port number
func TestPickRandomPort(t *testing.T) {
	port := PickRandomTCPPort()
	if port <= 1024 && port >= 65535 {
		t.Errorf("Expected 1024 < port < 65535 but got %d", port)
	}
}
