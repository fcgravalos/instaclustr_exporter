package mock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/common/log"
)

const (
	jsonStorageRelativePath     = "data"
	mockServerAddress           = "127.0.0.1:8082"
	notFoundResponse            = `{"status": 404, "message": "HTTP 404 Not Found", "link": "https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html"}`
	internalServerErrorResponse = `{"status": 500, "message": "HTTP Internal Server Error 500 Server", "link": "https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html"}`
)

var (
	jsonStoragePath       string
	instaclustrMockServer *mockServer
)

type mockServer struct {
	server           http.Server
	shutdownReq      chan bool
	shutdownReqCount uint32
}

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatalln("Could not get running directory")
	}
	jsonStoragePath = filepath.Join(filepath.Dir(filename), jsonStorageRelativePath)
}

func loadJSONFile(path string) ([]byte, error) {
	jsonBytes, err := ioutil.ReadFile(path)
	jsonData := bytes.Trim(jsonBytes, "\n")
	if err != nil {
		log.Errorf("Error reading file %s: %v", path, err)
		return nil, err
	}
	return jsonData, nil
}

func getClustersHandler(w http.ResponseWriter, r *http.Request) {
	var response interface{}
	jsonData, err := loadJSONFile(fmt.Sprintf("%s/listAllClusters.json", jsonStoragePath))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		jsonData = []byte(internalServerErrorResponse)
	}
	if err := json.Unmarshal(jsonData, &response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("Could not unmarshal json %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getClusterStatusHandler(w http.ResponseWriter, r *http.Request) {
	var response interface{}
	clusterID := path.Base(r.URL.String())
	jsonData, err := loadJSONFile(fmt.Sprintf("%s/%s/getClusterStatus.json", jsonStoragePath, clusterID))
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			jsonData = []byte(notFoundResponse)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			jsonData = []byte(internalServerErrorResponse)
		}
	}
	if err := json.Unmarshal(jsonData, &response); err != nil {
		log.Errorf("Could not unmarshal json %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getAllNodeMetricsHandler(w http.ResponseWriter, r *http.Request) {
	var response interface{}
	u, _ := url.Parse(r.URL.RequestURI())
	nodeID := path.Base(u.Path)
	jsonData, err := loadJSONFile(fmt.Sprintf("%s/%s/getAllNodeMetrics.json", jsonStoragePath, nodeID))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		jsonData = []byte(notFoundResponse)
	}
	if err := json.Unmarshal(jsonData, &response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("Could not unmarshal json %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func createServer() *mockServer {
	s := &mockServer{
		server: http.Server{
			Addr:         mockServerAddress,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		shutdownReq: make(chan bool),
	}
	router := mux.NewRouter()
	router.HandleFunc("/shutdown", s.shutdownHandler).Methods("GET")
	router.HandleFunc("/health", s.healthHandler).Methods("GET")

	provisioningAPIRouter := router.PathPrefix("/provisioning/v1").Subrouter()
	monitoringAPIRouter := router.PathPrefix("/monitoring/v1").Subrouter()

	//GET Methods
	provisioningAPIRouter.HandleFunc("", getClustersHandler).Methods("GET")
	provisioningAPIRouter.HandleFunc("/{id}", getClusterStatusHandler).Methods("GET")
	monitoringAPIRouter.HandleFunc("/nodes/{id}", getAllNodeMetricsHandler).Methods("GET")

	s.server.Handler = router

	return s
}

// WaitForServerUp allows external check if the Mock server is running by doing an HTTP GET to health endpoint
func WaitForServerUp() bool {
	up := false
	retries := 10
	waitIntervalSeconds := 5 * time.Second
	for !up && retries > 0 {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://"+"%s/health", mockServerAddress), nil)
		if err != nil {
			log.Errorf("Couldn't create healt check request: %v", err)
			retries--
			time.Sleep(waitIntervalSeconds)
			continue
		}
		resp, err := new(http.Client).Do(req)
		if err != nil {
			retries--
			time.Sleep(waitIntervalSeconds)
			continue
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			retries--
			time.Sleep(waitIntervalSeconds)
			continue
		}
		if string(body) == "OK" && resp.StatusCode == http.StatusOK {
			up = true
			break
		}
	}
	return up
}

// NewMockServer Starts a simple mock server for Instaclustr API
func NewMockServer() {
	s := createServer()
	go func() {
		log.Info("Starting InstaClustr API Mock server...")
		err := s.server.ListenAndServe()
		if err != nil {
			log.Errorf("Failed to start Mock Server: %v", err)
		}

	}()
	s.WaitForShutdown()
}

func (s *mockServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *mockServer) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Shutting down server"))

	//Do nothing if shutdown request already issued
	//if s.reqCount == 0 then set to 1, return true otherwise false
	if !atomic.CompareAndSwapUint32(&s.shutdownReqCount, 0, 1) {
		log.Infof("Shutdown through API call in progress...")
		return
	}

	go func() {
		s.shutdownReq <- true
	}()
}

func (s *mockServer) WaitForShutdown() {
	irqSig := make(chan os.Signal, 1)
	signal.Notify(irqSig, syscall.SIGINT, syscall.SIGTERM)

	//Wait interrupt or shutdown request through /shutdown
	select {
	case sig := <-irqSig:
		log.Infof("Shutdown request (signal: %v)", sig)
	case sig := <-s.shutdownReq:
		log.Infof("Shutdown request (/shutdown %v)", sig)
	}

	log.Infof("Stopping InstaClustr API Mock server ...")

	//Create shutdown context with 10 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//shutdown the server
	err := s.server.Shutdown(ctx)
	if err != nil {
		log.Errorf("Shutdown request error: %v", err)
	}
}
