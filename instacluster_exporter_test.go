package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fcgravalos/instaclustr_exporter/common"
	"github.com/fcgravalos/instaclustr_exporter/instaclustr"
	"github.com/fcgravalos/instaclustr_exporter/mock"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	exporterServer *common.Server
	mockServer     *common.Server
)

func setup(up chan bool) {
	// Picking up random port for InstaClustr API mock server
	mockServerPort := strconv.Itoa(common.PickRandomTCPPort())
	mockServerListenAddress := fmt.Sprintf("127.0.0.1:%s", mockServerPort)

	exporterServerPort := strconv.Itoa(common.PickRandomTCPPort())
	exporterServerListenAddress := fmt.Sprintf("127.0.0.1:%s", exporterServerPort)

	msOpts := common.ServerOptions{
		ListenAddress:    mockServerListenAddress,
		LivenessProbeURL: "/health",
		ShutdownURL:      "/shutdown",
		ReadTimeOut:      10 * time.Second,
		WriteTimeOut:     10 * time.Second,
	}

	sOpts := common.ServerOptions{
		ListenAddress:    exporterServerListenAddress, //InstaClustr Exporter Server,
		LivenessProbeURL: "/health",
		ShutdownURL:      "/shutdown",
		ReadTimeOut:      10 * time.Second,
		WriteTimeOut:     10 * time.Second,
	}

	icOpts := instaclustr.Config{
		Url:                fmt.Sprintf("http://"+"%s", mockServerListenAddress), //InstaClustr API Mock Server
		User:               "test",
		ProvisioningAPIKey: "test",
		MonitoringAPIKey:   "test",
	}
	exporterServer = NewExporter("/metrics", sOpts, icOpts)
	mockServer = mock.NewMockServer(msOpts)

	go func() {
		exporterServer.Start()
	}()
	go func() {
		mockServer.Start()
	}()
	go func(chan bool) {
		exporterServer.WaitForLiveness()
		mockServer.WaitForLiveness()
		up <- true
	}(up)
}

func tearDown() {
	exporterServer.GracefulShutdown()
	mockServer.GracefulShutdown()
}

func TestHomeHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(homeHandler)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	// Check the response body is what we expect.
	expected := `<html>
					<head><title>InstaClustr Exporter</title></head>
					<body>
					<h1>InstaClustr Exporter</h1>
					<p><a href="/metrics">Metrics</a></p>
					</body>
					</html>`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestHealthHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(exporterServer.LivenessProbeHandler)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := "OK"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestMetricsHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := prometheus.Handler()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	// Check the response body is what we expect.
	expected := `# HELP cassandra_cluster_nodes_count Number of nodes the cluster is composed
# TYPE cassandra_cluster_nodes_count gauge
cassandra_cluster_nodes_count{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01"} 1
# HELP cassandra_cluster_nodes_running_count Number of nodes running in the cluster
# TYPE cassandra_cluster_nodes_running_count gauge
cassandra_cluster_nodes_running_count{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01"} 1
# HELP cassandra_cluster_running Whether or not the cassandra cluster is running.
# TYPE cassandra_cluster_running gauge
cassandra_cluster_running{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01"} 1
# HELP cassandra_node_client_request_read_latency Average latency (us/1) per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client).
# TYPE cassandra_node_client_request_read_latency gauge
cassandra_node_client_request_read_latency{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 1462.5666666666664
# HELP cassandra_node_client_request_read_percentile 95th percentile (us) distribution per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client).
# TYPE cassandra_node_client_request_read_percentile gauge
cassandra_node_client_request_read_percentile{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 1866.1645999999998
# HELP cassandra_node_client_request_write_latency Average latency (us/1) per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client).
# TYPE cassandra_node_client_request_write_latency gauge
cassandra_node_client_request_write_latency{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 1293.5333333333335
# HELP cassandra_node_client_request_write_percentile 95th percentile (us) distribution per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client).
# TYPE cassandra_node_client_request_write_percentile gauge
cassandra_node_client_request_write_percentile{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 1669.6253
# HELP cassandra_node_compactions Number of pending compactions.
# TYPE cassandra_node_compactions gauge
cassandra_node_compactions{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 0
# HELP cassandra_node_cpu_utilization_percentage Current CPU utilisation as a percentage of total available. Maximum value is 100%, regardless of the number of cores on the node.
# TYPE cassandra_node_cpu_utilization_percentage gauge
cassandra_node_cpu_utilization_percentage{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 2.5884383
# HELP cassandra_node_disk_utilization_percentage Total disk space utilisation, by Cassandra, as a percentage of total available.
# TYPE cassandra_node_disk_utilization_percentage gauge
cassandra_node_disk_utilization_percentage{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 7.6197357
# HELP cassandra_node_reads_per_second Reads per second by Cassandra.
# TYPE cassandra_node_reads_per_second gauge
cassandra_node_reads_per_second{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 1.25
# HELP cassandra_node_repairs_active Number of pending repair tasks.
# TYPE cassandra_node_repairs_active gauge
cassandra_node_repairs_active{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 0
# HELP cassandra_node_repairs_pending Number of pending repair tasks.
# TYPE cassandra_node_repairs_pending gauge
cassandra_node_repairs_pending{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 0
# HELP cassandra_node_running Whether or not a single node is running
# TYPE cassandra_node_running gauge
cassandra_node_running{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 1
# HELP cassandra_node_writes_per_second Writes per second by Cassandra.
# TYPE cassandra_node_writes_per_second gauge
cassandra_node_writes_per_second{clusterId="cluster-uuid-1",clusterName="MOCKED_CLUSTER_01",nodeId="node-uuid-1",nodePrivateIp="e.f.g.h",nodePublicIp="a.b.c.d",rack="MOCKED_RACK_01"} 1.25
`
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestMain(m *testing.M) {
	up := make(chan bool)
	setup(up)
	<-up
	m.Run()
	tearDown()
}
