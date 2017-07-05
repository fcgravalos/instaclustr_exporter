package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fcgravalos/instaclustr_exporter/collector"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

type serverOptions struct {
	listenAddress string
	telemetryPath string
	readTimeout   time.Duration
	writeTimeout  time.Duration
}

type exporterHTTPServer struct {
	server           http.Server
	shutdownReq      chan bool
	shutdownReqCount uint32
}

var serverOpts serverOptions

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<html>
					<head><title>InstaClustr Exporter</title></head>
					<body>
					<h1>InstaClustr Exporter</h1>
					<p><a href="/metrics">Metrics</a></p>
					</body>
					</html>`))
}

func (e *exporterHTTPServer) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Shutting down server"))

	//Do nothing if shutdown request already issued
	//if s.reqCount == 0 then set to 1, return true otherwise false
	if !atomic.CompareAndSwapUint32(&e.shutdownReqCount, 0, 1) {
		log.Infof("Shutdown through API call in progress...")
		return
	}

	go func() {
		e.shutdownReq <- true
	}()
}

func createServer() *exporterHTTPServer {
	s := &exporterHTTPServer{
		server: http.Server{
			Addr:         serverOpts.listenAddress,
			ReadTimeout:  serverOpts.readTimeout,
			WriteTimeout: serverOpts.writeTimeout,
		},
		shutdownReq: make(chan bool),
	}
	router := mux.NewRouter()
	router.HandleFunc("/", homeHandler).Methods("GET")
	router.HandleFunc("/shutdown", s.shutdownHandler).Methods("GET")
	router.HandleFunc("/health", healthHandler).Methods("GET")
	router.Handle(serverOpts.telemetryPath, prometheus.Handler()).Methods("GET")
	s.server.Handler = router

	return s
}

func (e *exporterHTTPServer) waitForShutdown() {
	irqSig := make(chan os.Signal, 1)
	signal.Notify(irqSig, syscall.SIGINT, syscall.SIGTERM)

	//Wait interrupt or shutdown request through /shutdown
	select {
	case sig := <-irqSig:
		log.Infof("Shutdown request (signal: %v)", sig)
	case sig := <-e.shutdownReq:
		log.Infof("Shutdown request (/shutdown %v)", sig)
	}

	log.Infof("Stopping http server ...")

	//Create shutdown context with 10 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//shutdown the server
	err := e.server.Shutdown(ctx)
	if err != nil {
		log.Errorf("Shutdown request error: %v", err)
	} else {
		log.Infoln("Server stopped")
	}
}

// WaitForServerUp waits for the exporter health endpoint to be up & running
func WaitForServerUp() bool {
	up := false
	retries := 10
	waitIntervalSeconds := 5 * time.Second
	wait := func() {
		retries--
		time.Sleep(waitIntervalSeconds)
	}
	// TODO Implement exponential back-off, in every loop we increment the wait Interval
	for !up && retries > 0 {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://"+"%s/health", serverOpts.listenAddress), nil)
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
			up = true
			break
		}
	}
	return up
}

// StartExporter runs the http server
func StartExporter() {
	exp := collector.NewExporter()
	prometheus.MustRegister(exp)
	// start httpServer
	s := createServer()
	log.Infof("InstaClustr Exporter started on %s\n", serverOpts.listenAddress)
	go func() {
		err := s.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting InstaClustr Exporter server: %v", err)
		}
	}()
	s.waitForShutdown()
}

func main() {
	var (
		showVersion = flag.Bool("version", false, "Print version information.")

		//testingMode   = flag.Bool("instaclustr.testing-mode", false, "Determines if a mock server is used or the real API.")
		//opts          = instaclustrOpts{}
	)
	flag.StringVar(&serverOpts.listenAddress, "web.listen-address", ":9999", "Address to listen on for web interface and telemetry.")
	flag.DurationVar(&serverOpts.readTimeout, "web.read-timeout", 10*time.Second, "Read/Write Timeout")
	flag.DurationVar(&serverOpts.writeTimeout, "web.write-timeout", 10*time.Second, "Read/Write Timeout")
	flag.StringVar(&serverOpts.telemetryPath, "web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	/*
		flag.StringVar(&opts.user, "instaclustr.user", "", "User for InstaClustr API")
		flag.StringVar(&opts.provisioningApiKey, "instaclustr.provisioning-apikey", "", "Key for the provisioning API")
		flag.StringVar(&opts.provisioningApiKey, "instaclustr.monitoring-apikey", "", "Key for the provisioning API")
	*/
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Print("instaclustr_exporter"))
		os.Exit(0)
	}
	fmt.Println(serverOpts)
	StartExporter()
}
