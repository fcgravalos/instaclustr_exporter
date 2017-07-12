package common

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

var testServer *Server

func newTestServer(serverOpts ServerOptions) *Server {
	s := NewServer("test_server", serverOpts)
	router := mux.NewRouter()
	router.HandleFunc(serverOpts.ShutdownURL, s.ShutDownHandler).Methods("GET")
	router.HandleFunc(serverOpts.LivenessProbeURL, s.LivenessProbeHandler).Methods("GET")
	s.HTTPServer.Handler = router
	return s
}

func setup(up chan bool) {
	testServerPort := strconv.Itoa(PickRandomTCPPort())
	testServerListenAddress := fmt.Sprintf("127.0.0.1:%s", testServerPort)

	sOpts := ServerOptions{
		ListenAddress:    testServerListenAddress,
		LivenessProbeURL: "/health",
		ShutdownURL:      "/shutdown",
		ReadTimeOut:      10 * time.Second,
		WriteTimeOut:     10 * time.Second,
	}
	testServer = newTestServer(sOpts)

	go func() {
		testServer.Start()
	}()
	go func(up chan bool) {
		testServer.WaitForLiveness()
		up <- true
	}(up)
}

func tearDown() {
	testServer.GracefulShutdown()
}

func TestWaitForLiveness(t *testing.T) {
	if !testServer.WaitForLiveness() {
		t.Errorf("Waiting for liveness returned a false value")
	}
}

func TestMain(m *testing.M) {
	up := make(chan bool)
	setup(up)
	<-up
	m.Run()
	tearDown()
}
