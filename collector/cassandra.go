package collector

import (
	"strconv"
	"strings"
	"sync"

	"encoding/json"

	"github.com/fcgravalos/instaclustr_exporter/instaclustr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	namespace = "cassandra"
)

// InstaClustr API handlers
var provisioningClient *instaclustr.ProvisioningClient
var monitoringClient *instaclustr.MonitoringClient

var allNodeMetricsQuery = []string{
	//"n::nodeStatus",         //Whether Cassandra is available on the node. Returns a "warn" value, if no check in has been logged in the last 30 seconds.
	"n::cpuUtilization",     //Current CPU utilisation as a percentage of total available. Maximum value is 100%, regardless of the number of cores on the node.
	"n::diskUtilization",    //Total disk space utilisation, by Cassandra, as a percentage of total available.
	"n::cassandraReads",     //Reads per second by Cassandra.
	"n::cassandraWrites",    //Writes per second by Cassandra.
	"n::compactions",        //Number of pending compactions.
	"n::repairs",            //Number of active and pending repair tasks.
	"n::clientRequestRead",  //95th percentile distribution and average latency per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client).
	"n::clientRequestWrite", //95th percentile distribution and average latency per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client).
}

// Metric descriptors
var (
	clusterRunning = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "running"),
		"Whether or not the cassandra cluster is running.",
		[]string{"clusterId", "clusterName"},
		nil,
	)
	clusterNodesCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "nodes_count"),
		"Number of nodes the cluster is composed",
		[]string{"clusterId", "clusterName"},
		nil,
	)
	clusterNodesRunningCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "nodes_running_count"),
		"Number of nodes running in the cluster",
		[]string{"clusterId", "clusterName"},
		nil,
	)
	nodeRunning = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "running"),
		"Whether or not a single node is running",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeCPUUtilizationPercentage = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "cpu_utilization_percentage"),
		"Current CPU utilisation as a percentage of total available. Maximum value is 100%, regardless of the number of cores on the node.",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeDiskUtilizationPercentage = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "disk_utilization_percentage"),
		"Total disk space utilisation, by Cassandra, as a percentage of total available.",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeCassandraReadsPerSecond = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "reads_per_second"),
		"Reads per second by Cassandra.",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeCassandraWritesPerSecond = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "writes_per_second"),
		"Writes per second by Cassandra.",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeCassandraCompactions = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "compactions"),
		"Number of pending compactions.",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeCassandraRepairsPending = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "repairs_pending"),
		"Number of pending repair tasks.",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeCassandraRepairsActive = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "repairs_active"),
		"Number of pending repair tasks.",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeClientRequestReadLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "client_request_read_latency"),
		"Average latency (us/1) per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client).",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeClientRequestWriteLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "client_request_write_latency"),
		"Average latency (us/1) per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client).",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeClientRequestReadPercentile = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "client_request_read_percentile"),
		"95th percentile (us) distribution per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client).",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeClientRequestWritePercentile = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "client_request_write_percentile"),
		"95th percentile (us) distribution per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client).",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
)

type cluster struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	NodeCount        int    `json:"nodeCount"`
	RunningNodeCount int    `json:"runningNodeCount"`
	DerivedStatus    string `json:"derivedStatus"`
}

type node struct {
	ID             string `json:"id"`
	Size           string `json:"size"`
	Rack           string `json:"rack"`
	PublicIP       string `json:"publicAddress"`
	PrivateIP      string `json:"privateAddress"`
	Status         string `json:"nodeStatus"`
	SparkMaster    bool   `json:"sparkMaster"`
	SparkJobserver bool   `json:"sparkJobserver"`
	Zeppelin       bool   `json:"zeppelin"`
}

type datacentres struct {
	Dcs []datacentre `json:"dataCentres"`
}

type datacentre struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Provider   string                 `json:"provider"`
	CDCNetwork map[string]interface{} `json:"cdcNetwork"`
	Nodes      []node                 `json:"nodes"`
}

type metrics struct {
	Metrics []metric `json:"payload"`
}

type metric struct {
	Name   string        `json:"metric"`
	Type   string        `json:"type"`
	Unit   string        `json:"unit"`
	Values []metricValue `json:"values"`
}

type metricValue struct {
	Value string `json:"value"`
	Time  string `json:"time"`
}

// Exporter types defines a InstaClustr Exporter
type Exporter struct {
	provisioningClient *instaclustr.ProvisioningClient
	monitoringClient   *instaclustr.MonitoringClient
}

// NewExporter creates new InstaClustr Cassandra Exporter
func NewExporter() *Exporter {
	return &Exporter{
		provisioningClient: instaclustr.NewProvisioningClient(),
		monitoringClient:   instaclustr.NewMonitoringClient(),
	}
}

func clusterHealthCollector(c cluster, ch chan<- prometheus.Metric) {
	if c.DerivedStatus == "RUNNING" {
		ch <- prometheus.MustNewConstMetric(
			clusterRunning,
			prometheus.GaugeValue,
			1,
			c.ID,
			c.Name,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			clusterRunning,
			prometheus.GaugeValue,
			0,
			c.ID,
			c.Name,
		)
	}
	ch <- prometheus.MustNewConstMetric(
		clusterNodesCount,
		prometheus.GaugeValue,
		float64(c.NodeCount),
		c.ID,
		c.Name,
	)
	ch <- prometheus.MustNewConstMetric(
		clusterNodesRunningCount,
		prometheus.GaugeValue,
		float64(c.RunningNodeCount),
		c.ID,
		c.Name,
	)
}

func nodeHealthCollector(c cluster, n node, ch chan<- prometheus.Metric) {
	if n.Status == "RUNNING" {
		ch <- prometheus.MustNewConstMetric(
			nodeRunning,
			prometheus.GaugeValue,
			1,
			c.ID,
			c.Name,
			n.ID,
			n.PublicIP,
			n.PrivateIP,
			n.Rack,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			nodeRunning,
			prometheus.GaugeValue,
			0,
			c.ID,
			c.Name,
			n.ID,
			n.PublicIP,
			n.PrivateIP,
			n.Rack,
		)
	}
}

// nodeMetricsCollector gathers all Node metrics but the status
func nodeMetricsCollector(c cluster, n node, ms []metrics, ch chan<- prometheus.Metric) {

	for _, mc := range ms {
		for _, m := range mc.Metrics {
			value, err := strconv.ParseFloat(m.Values[0].Value, 64)
			if err != nil {
				log.Errorf("Error parsing value metric %s : %s", m.Name, m.Values[0].Value)
				value = 0
			}
			switch m.Name {

			case "cpuUtilization":
				ch <- prometheus.MustNewConstMetric(
					nodeCPUUtilizationPercentage,
					prometheus.GaugeValue,
					value,
					c.ID,
					c.Name,
					n.ID,
					n.PublicIP,
					n.PrivateIP,
					n.Rack,
				)

			case "diskUtilization":
				ch <- prometheus.MustNewConstMetric(
					nodeDiskUtilizationPercentage,
					prometheus.GaugeValue,
					value,
					c.ID,
					c.Name,
					n.ID,
					n.PublicIP,
					n.PrivateIP,
					n.Rack,
				)

			case "cassandraReads":
				ch <- prometheus.MustNewConstMetric(
					nodeCassandraReadsPerSecond,
					prometheus.GaugeValue,
					value,
					c.ID,
					c.Name,
					n.ID,
					n.PublicIP,
					n.PrivateIP,
					n.Rack,
				)

			case "cassandraWrites":
				ch <- prometheus.MustNewConstMetric(
					nodeCassandraWritesPerSecond,
					prometheus.GaugeValue,
					value,
					c.ID,
					c.Name,
					n.ID,
					n.PublicIP,
					n.PrivateIP,
					n.Rack,
				)

			case "compactions":
				ch <- prometheus.MustNewConstMetric(
					nodeCassandraCompactions,
					prometheus.GaugeValue,
					value,
					c.ID,
					c.Name,
					n.ID,
					n.PublicIP,
					n.PrivateIP,
					n.Rack,
				)

			case "repairs":
				if m.Type == "pendingtasks" {
					ch <- prometheus.MustNewConstMetric(
						nodeCassandraRepairsPending,
						prometheus.GaugeValue,
						value,
						c.ID,
						c.Name,
						n.ID,
						n.PublicIP,
						n.PrivateIP,
						n.Rack,
					)
				} else if m.Type == "activetasks" {
					ch <- prometheus.MustNewConstMetric(
						nodeCassandraRepairsActive,
						prometheus.GaugeValue,
						value,
						c.ID,
						c.Name,
						n.ID,
						n.PublicIP,
						n.PrivateIP,
						n.Rack,
					)
				} else {
					log.Warnf("Unknown n::%s metric type %s", m.Name, m.Type)
				}

			case "clientRequestRead":
				if m.Type == "latency_per_operation" {
					ch <- prometheus.MustNewConstMetric(
						nodeClientRequestReadLatency,
						prometheus.GaugeValue,
						value,
						c.ID,
						c.Name,
						n.ID,
						n.PublicIP,
						n.PrivateIP,
						n.Rack,
					)
				} else if m.Type == "95thPercentile" {
					ch <- prometheus.MustNewConstMetric(
						nodeClientRequestReadPercentile,
						prometheus.GaugeValue,
						value,
						c.ID,
						c.Name,
						n.ID,
						n.PublicIP,
						n.PrivateIP,
						n.Rack,
					)
				} else {
					log.Warnf("Unknown n::%s metric type %s", m.Name, m.Type)
				}

			case "clientRequestWrite":
				if m.Type == "latency_per_operation" {
					ch <- prometheus.MustNewConstMetric(
						nodeClientRequestWriteLatency,
						prometheus.GaugeValue,
						value,
						c.ID,
						c.Name,
						n.ID,
						n.PublicIP,
						n.PrivateIP,
						n.Rack,
					)
				} else if m.Type == "95thPercentile" {
					ch <- prometheus.MustNewConstMetric(
						nodeClientRequestWritePercentile,
						prometheus.GaugeValue,
						value,
						c.ID,
						c.Name,
						n.ID,
						n.PublicIP,
						n.PrivateIP,
						n.Rack,
					)
				} else {
					log.Warnf("Unknown n::%s metric type %s", m.Name, m.Type)
				}
			}
		}
	}
}

// Describe describes all the metrics ever exported by the Consul exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- clusterRunning
	ch <- clusterNodesCount
	ch <- clusterNodesRunningCount
	ch <- nodeRunning
	ch <- nodeCPUUtilizationPercentage
	ch <- nodeDiskUtilizationPercentage
	ch <- nodeCassandraReadsPerSecond
	ch <- nodeCassandraWritesPerSecond
	ch <- nodeCassandraCompactions
	ch <- nodeCassandraRepairsPending
	ch <- nodeCassandraRepairsActive
	ch <- nodeClientRequestReadLatency
	ch <- nodeClientRequestWriteLatency
	ch <- nodeClientRequestReadPercentile
	ch <- nodeClientRequestWritePercentile
}

// Collect fetches the stats from configured Consul location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	clusters := []cluster{}
	dcs := new(datacentres)
	ms := []metrics{}
	wg := new(sync.WaitGroup)

	// Fetching clusters list
	if err := json.Unmarshal(e.provisioningClient.GetClusters(), &clusters); err != nil {
		log.Errorf("Couldn't get clusters: %v", err)
		return
	}

	for _, c := range clusters {
		wg.Add(1)
		go func(c cluster) {
			defer wg.Done()
			clusterHealthCollector(c, ch)

			// Queryng status of the cluster, gathers the list of Datacentres
			if err := json.Unmarshal(e.provisioningClient.GetClusterStatus(c.ID), &dcs); err != nil {
				log.Errorf("Couldn't get cluster %s datacentres: %v", c.ID, err)
				return
			}

			for _, dc := range dcs.Dcs {
				for _, n := range dc.Nodes {
					nodeHealthCollector(c, n, ch)

					// Fetch all metrics from node
					if err := json.Unmarshal(e.monitoringClient.GetNodeMetric(n.ID, strings.Join(allNodeMetricsQuery, ",")), &ms); err != nil {
						log.Errorf("Could not gather any metric: %v\n", err)
						return
					}

					// Collecting node metrics
					nodeMetricsCollector(c, n, ms, ch)
				}
			}
		}(c)
	}
	// We don't close the channel, prometheus does the job
	wg.Wait()
}
