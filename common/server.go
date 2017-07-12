package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/common/log"
)

// ServerOptions defines the server configuration
type ServerOptions struct {
	ListenAddress    string
	LivenessProbeURL string
	ShutdownURL      string
	ReadTimeOut      time.Duration
	WriteTimeOut     time.Duration
}

// Server represents a server type
type Server struct {
	Name             string
	HTTPServer       http.Server
	LivenessProbeURL string
	ShutdownURL      string
	ShutdownReq      chan bool
	ShutdownReqCount uint32
}

// LivenessProbeHandler handles healt-check requests to LivenessProbeURL
func (s *Server) LivenessProbeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// ShutDownHandler provides a graceful shutdown via API
func (s *Server) ShutDownHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Shutting Down... bye! :)"))
	//Do nothing if shutdown request already issued
	//if s.reqCount == 0 then set to 1, return true otherwise false
	if !atomic.CompareAndSwapUint32(&s.ShutdownReqCount, 0, 1) {
		log.Infof("Shutdown through API call in progress...")
		return
	}

	go func() {
		s.ShutdownReq <- true
	}()
}

// WaitForShutDown blocks until a shutdown request gets to the server
func (s *Server) WaitForShutDown() {
	irqSig := make(chan os.Signal, 1)
	signal.Notify(irqSig, syscall.SIGINT, syscall.SIGTERM)

	//Wait interrupt or shutdown request through /shutdown
	select {
	case sig := <-irqSig:
		log.Infof("[%s] Shutdown request (signal: %v)", s.Name, sig)
	case sig := <-s.ShutdownReq:
		log.Infof("[%s] Shutdown HTTP request (http: %v)", s.Name, sig)
	}

	log.Infof("[%s] Stopping server...", s.Name)

	//Create shutdown context with 10 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//shutdown the server
	err := s.HTTPServer.Shutdown(ctx)
	if err != nil {
		log.Errorf("[%s] Shutdown request error: %v", s.Name, err)
	} else {
		log.Infof("[%s] Server stopped", s.Name)
	}
}

// WaitForLiveness blocks un till the server is alive
func (s *Server) WaitForLiveness() bool {
	live := false
	retries := 10
	waitIntervalSeconds := 1 * time.Second
	wait := func() {
		retries--
		time.Sleep(waitIntervalSeconds)
	}
	// TODO Implement exponential back-off, in every loop we increment the wait Interval
	for !live && retries > 0 {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://"+"%s/%s", s.HTTPServer.Addr, strings.Trim(s.LivenessProbeURL, "/")), nil)
		if err != nil {
			wait()
			continue
		}
		resp, err := new(http.Client).Do(req)
		if err != nil {
			wait()
			continue
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			wait()
			continue
		}
		if string(body) == "OK" && resp.StatusCode == http.StatusOK {
			live = true
			break
		}
	}
	return live
}

// Start starts the server and blocks until it's shut down
func (s *Server) Start() {
	go func() {
		if err := s.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[%s] Could not start server", s.Name)
		}
	}()
	log.Infof("[%s] started on %s", s.Name, s.HTTPServer.Addr)
	s.WaitForShutDown()
}

// GracefulShutdown shut down provides a safe mechanism tu shut the server down
func (s *Server) GracefulShutdown() {
	log.Infof("Shutting down %s", s.Name)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://"+"%s/%s", s.HTTPServer.Addr, strings.Trim(s.ShutdownURL, "/")), nil)
	if err != nil {
		log.Errorf("Could not send shutdown request to %s Server: %v", s.Name, err)
	}
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Error sending request: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("Could not read shutdown response: %v", body)
	}
	log.Infof("Server status: %s", string(body))
}

// NewServer Builds a new server
func NewServer(name string, opts ServerOptions) *Server {
	return &Server{
		Name: name,
		HTTPServer: http.Server{
			Addr:         opts.ListenAddress,
			ReadTimeout:  opts.ReadTimeOut,
			WriteTimeout: opts.WriteTimeOut,
		},
		LivenessProbeURL: opts.LivenessProbeURL,
		ShutdownURL:      opts.ShutdownURL,
		ShutdownReq:      make(chan bool),
	}
}
