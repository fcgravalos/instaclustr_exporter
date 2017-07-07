package instaclustr

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/prometheus/common/log"
)

const (
	defaultURL              = "https://api.instaclustr.com"
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

type Config struct {
	Url                string
	User               string
	ProvisioningAPIKey string
	MonitoringAPIKey   string
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

func createInstaClustrClient(instaclustrURL string, user string, apiKey string, apiEndpoint string, apiVersion string) instaclustrClient {
	var stringURL string
	parsedURL, err := url.Parse(instaclustrURL)
	if err != nil {
		log.Errorf("Parsing error: %v", err)
		stringURL = defaultURL
	} else if parsedURL.String() == "" {
		stringURL = defaultURL
	} else {
		stringURL = parsedURL.String()
	}
	return instaclustrClient{
		url:         stringURL,
		user:        user,
		APIKey:      apiKey,
		APIEndpoint: apiEndpoint,
		APIVersion:  apiVersion,
		client:      &http.Client{},
	}
}

// NewProvisioningClient creates a ProvisioningClient
func NewProvisioningClient(config Config) *ProvisioningClient {
	ic := createInstaClustrClient(config.Url, config.User, config.ProvisioningAPIKey, provisioningAPIEndpoint, provisioningAPIVersion)
	pc := ProvisioningClient(ic)
	return &pc
}

// NewMonitoringClient creates a MonitoringClient
func NewMonitoringClient(config Config) *MonitoringClient {
	ic := createInstaClustrClient(config.Url, config.User, config.MonitoringAPIKey, monitoringAPIEndpoint, monitoringAPIVersion)
	mc := MonitoringClient(ic)
	return &mc
}

func (c instaclustrClient) sendRequest(req *http.Request) ([]byte, error) {
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

	data, err := instaclustrClient(c).sendRequest(req)
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

	data, err := instaclustrClient(c).sendRequest(req)
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

	data, err := instaclustrClient(c).sendRequest(req)
	if err != nil {
		log.Errorf("Error querying %s: %s", req.RequestURI, err.Error())
		return nil
	}
	return data
}
