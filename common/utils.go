package common

import (
	"net"

	"github.com/prometheus/common/log"
)

// PickRandomTCPPort picks free TCP Port from localhost
func PickRandomTCPPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		log.Fatalf("Could not resolve address: %v", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatalf("Could not setup port %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
