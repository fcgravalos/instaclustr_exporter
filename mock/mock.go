package mock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/fcgravalos/instaclustr_exporter/common"
	"github.com/gorilla/mux"
	"github.com/prometheus/common/log"
)

const (
	jsonStorageRelativePath     = "data"
	notFoundResponse            = `{"status": 404, "message": "HTTP 404 Not Found", "link": "https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html"}`
	internalServerErrorResponse = `{"status": 500, "message": "HTTP Internal Server Error 500 Server", "link": "https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html"}`
)

var (
	jsonStoragePath string
)

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

// NewMockServer creates a new mock server for the InstaClustr API
func NewMockServer(serverOpts common.ServerOptions) *common.Server {

	// start httpServer
	s := common.NewServer("instaclustr_mock_server", serverOpts)
	router := mux.NewRouter()
	router.HandleFunc(serverOpts.ShutdownURL, s.ShutDownHandler).Methods("GET")
	router.HandleFunc(serverOpts.LivenessProbeURL, s.LivenessProbeHandler).Methods("GET")

	provisioningAPIRouter := router.PathPrefix("/provisioning/v1").Subrouter()
	monitoringAPIRouter := router.PathPrefix("/monitoring/v1").Subrouter()

	//GET Methods
	provisioningAPIRouter.HandleFunc("", getClustersHandler).Methods("GET")
	provisioningAPIRouter.HandleFunc("/{id}", getClusterStatusHandler).Methods("GET")
	monitoringAPIRouter.HandleFunc("/nodes/{id}", getAllNodeMetricsHandler).Methods("GET")
	s.HTTPServer.Handler = router
	return s
}
