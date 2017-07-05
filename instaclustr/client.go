package instaclustr

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/prometheus/common/log"
)

const (
	//instaclustrURL          = "https://api.instaclustr.com"
	instaclustrURL          = "http://127.0.0.1:8082"
	provisioningAPIEndpoint = "provisioning"
	monitoringAPIEndpoint   = "monitoring"
	provisioningAPIVersion  = "v1"
	monitoringAPIVersion    = "v1"
)

var (
	user               string
	provisioningAPIKey string
	monitoringAPIKey   string
)

func init() {
	if os.Getenv("INSTACLUSTR_USER") != "" {
		user = os.Getenv("INSTACLUSTR_USER")
	}

	if os.Getenv("PROVISIONING_API_KEY") != "" {
		provisioningAPIKey = os.Getenv("PROVISIONING_API_KEY")
	}

	if os.Getenv("MONITORING_API_KEY") != "" {
		monitoringAPIKey = os.Getenv("MONITORING_API_KEY")
	}
}

type instaclustrClient struct {
	url         string
	user        string
	APIKey      string
	APIEndpoint string
	APIVersion  string
	client      *http.Client
}

// ProvisioningClient is a client for InstaClustr Provisioning API
type ProvisioningClient instaclustrClient

// MonitoringClient is a client for InstaClustr Monitoring API
type MonitoringClient instaclustrClient

// NewProvisioningClient creates a ProvisioningClient
func NewProvisioningClient() *ProvisioningClient {
	return &ProvisioningClient{
		url:         instaclustrURL,
		user:        user,
		APIKey:      provisioningAPIKey,
		APIEndpoint: provisioningAPIEndpoint,
		APIVersion:  provisioningAPIVersion,
		client:      &http.Client{},
	}
}

// NewMonitoringClient creates a MonitoringClient
func NewMonitoringClient() *MonitoringClient {
	return &MonitoringClient{
		url:         instaclustrURL,
		user:        user,
		APIKey:      monitoringAPIKey,
		APIEndpoint: monitoringAPIEndpoint,
		APIVersion:  monitoringAPIVersion,
		client:      &http.Client{},
	}
}

func sendRequest(c *instaclustrClient, req *http.Request) ([]byte, error) {
	req.SetBasicAuth(c.user, c.APIKey)
	resp, err := c.client.Do(req)
	if err != nil {
		log.Errorf("Error sending request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error reading response body: %v", err)
		return nil, err
	}
	return data, err
}

// GetClusters returns the list of Cassandra clusters
func (c ProvisioningClient) GetClusters() []byte {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s", c.url, c.APIEndpoint, c.APIVersion), nil)
	if err != nil {
		log.Errorf("Error building GetClusters request: %v", err)
		return nil
	}

	ic := instaclustrClient(c)
	data, err := sendRequest(&ic, req)
	if err != nil {
		log.Errorf("Error querying %s: %s", req.RequestURI, err.Error())
		return nil
	}
	return data
}

// GetClusterStatus returns a list of cluster attributes, datacentres and its nodes
func (c ProvisioningClient) GetClusterStatus(clusterID string) []byte {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/%s/%s/%s",
			c.url,
			c.APIEndpoint,
			c.APIVersion,
			clusterID,
		),
		nil)

	if err != nil {
		log.Errorf("Error building GetClusterStatus request: %v", err)
		return nil
	}

	ic := instaclustrClient(c)
	data, err := sendRequest(&ic, req)
	if err != nil {
		log.Errorf("Error querying %s: %s", req.RequestURI, err.Error())
		return nil
	}
	return data
}

// GetNodeMetric returns metrics from a node in a specific cluster
func (c MonitoringClient) GetNodeMetric(nodeID string, metric string) []byte {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/%s/%s/nodes/%s?metrics=%s",
			c.url,
			c.APIEndpoint,
			c.APIVersion,
			nodeID,
			metric,
		),
		nil)
	if err != nil {
		log.Errorf("Error building GetNodeMetric request: %v", err)
		return nil
	}

	ic := instaclustrClient(c)
	data, err := sendRequest(&ic, req)
	if err != nil {
		log.Errorf("Error querying %s: %s", req.RequestURI, err.Error())
		return nil
	}
	return data
}
