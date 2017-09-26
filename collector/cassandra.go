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
	namespace         = "cassandra"
	usTosecondsFactor = 1e-06
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
	clusterInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "info"),
		"A mapping between the clusterId and clusterName",
		[]string{"clusterId", "clusterName"},
		nil,
	)
	clusterRunning = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "running"),
		"Whether or not the cassandra cluster is running.",
		[]string{"clusterId"},
		nil,
	)
	// We don't name it with _count, because in Prometheus this would be a Summary/Histogram.
	// In our case, we are just grabbing the value from InstaClustr API
	clusterNodesCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "nodes"),
		"Number of nodes the cluster is composed",
		[]string{"clusterId"},
		nil,
	)
	clusterNodesRunningCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "nodes_running"),
		"Number of nodes running in the cluster",
		[]string{"clusterId"},
		nil,
	)
	nodeInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "info"),
		"A mapping between nodeId with its IPs, racks and cluster",
		[]string{"clusterId", "clusterName", "nodeId", "nodePublicIp", "nodePrivateIp", "rack"},
		nil,
	)
	nodeRunning = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "running"),
		"Whether or not a single node is running",
		[]string{"nodeId"},
		nil,
	)
	nodeCPUUtilizationPercentage = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "cpu_utilization_percentage"),
		"Current CPU utilisation as a percentage of total available. Maximum value is 100%, regardless of the number of cores on the node.",
		[]string{"nodeId"},
		nil,
	)
	nodeDiskUtilizationPercentage = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "disk_utilization_percentage"),
		"Total disk space utilisation, by Cassandra, as a percentage of total available.",
		[]string{"nodeId"},
		nil,
	)
	nodeCassandraReadsPerSecond = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "reads_per_second"),
		"Reads per second by Cassandra.",
		[]string{"nodeId"},
		nil,
	)
	nodeCassandraWritesPerSecond = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "writes_per_second"),
		"Writes per second by Cassandra.",
		[]string{"nodeId"},
		nil,
	)
	nodeCassandraCompactions = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "compactions"),
		"Number of pending compactions.",
		[]string{"nodeId"},
		nil,
	)
	nodeCassandraRepairsPending = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "repairs_pending"),
		"Number of pending repair tasks.",
		[]string{"nodeId"},
		nil,
	)
	nodeCassandraRepairsActive = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "repairs_active"),
		"Number of pending repair tasks.",
		[]string{"nodeId"},
		nil,
	)
	nodeClientRequestReadLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "client_request_read_latency"),
		"Average latency (s/1) per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client).",
		[]string{"nodeId"},
		nil,
	)
	nodeClientRequestWriteLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "client_request_write_latency"),
		"Average latency (s/1) per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client).",
		[]string{"nodeId"},
		nil,
	)
	nodeClientRequestReadPercentile = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "client_request_read_percentile95"),
		"95th percentile (s) distribution per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client).",
		[]string{"nodeId"},
		nil,
	)
	nodeClientRequestWritePercentile = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "client_request_write_percentile95"),
		"95th percentile (s) distribution per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client).",
		[]string{"nodeId"},
		nil,
	)
)

type cluster struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	NodeCount        float64 `json:"nodeCount"`
	RunningNodeCount float64 `json:"runningNodeCount"`
	DerivedStatus    string  `json:"derivedStatus"`
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

// NewExporter creates new InstaClustr Exporter
func NewExporter(instaclustrCfg instaclustr.Config) *Exporter {
	// NewExporter creates new InstaClustr Cassandra Exporter
	return &Exporter{
		provisioningClient: instaclustr.NewProvisioningClient(instaclustrCfg),
		monitoringClient:   instaclustr.NewMonitoringClient(instaclustrCfg),
	}
}

func clusterInfoCollector(c cluster, ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		clusterInfo,
		prometheus.CounterValue,
		1,
		c.ID,
		c.Name,
	)
}

func clusterHealthCollector(c cluster, ch chan<- prometheus.Metric) {
	if c.DerivedStatus == "RUNNING" {
		ch <- prometheus.MustNewConstMetric(
			clusterRunning,
			prometheus.GaugeValue,
			1,
			c.ID,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			clusterRunning,
			prometheus.GaugeValue,
			0,
			c.ID,
		)
	}
	ch <- prometheus.MustNewConstMetric(
		clusterNodesCount,
		prometheus.GaugeValue,
		c.NodeCount,
		c.ID,
	)
	ch <- prometheus.MustNewConstMetric(
		clusterNodesRunningCount,
		prometheus.GaugeValue,
		c.RunningNodeCount,
		c.ID,
	)
}

func nodeInfoCollector(c cluster, n node, ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		nodeInfo,
		prometheus.CounterValue,
		1,
		c.ID,
		c.Name,
		n.ID,
		n.PublicIP,
		n.PrivateIP,
		n.Rack,
	)
}

func nodeHealthCollector(c cluster, n node, ch chan<- prometheus.Metric) {
	if n.Status == "RUNNING" {
		ch <- prometheus.MustNewConstMetric(
			nodeRunning,
			prometheus.GaugeValue,
			1,
			n.ID,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			nodeRunning,
			prometheus.GaugeValue,
			0,
			n.ID,
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
					n.ID,
				)

			case "diskUtilization":
				ch <- prometheus.MustNewConstMetric(
					nodeDiskUtilizationPercentage,
					prometheus.GaugeValue,
					value,
					n.ID,
				)

			case "cassandraReads":
				ch <- prometheus.MustNewConstMetric(
					nodeCassandraReadsPerSecond,
					prometheus.GaugeValue,
					value,
					n.ID,
				)

			case "cassandraWrites":
				ch <- prometheus.MustNewConstMetric(
					nodeCassandraWritesPerSecond,
					prometheus.GaugeValue,
					value,
					n.ID,
				)

			case "compactions":
				ch <- prometheus.MustNewConstMetric(
					nodeCassandraCompactions,
					prometheus.GaugeValue,
					value,
					n.ID,
				)

			case "repairs":
				if m.Type == "pendingtasks" {
					ch <- prometheus.MustNewConstMetric(
						nodeCassandraRepairsPending,
						prometheus.GaugeValue,
						value,
						n.ID,
					)
				} else if m.Type == "activetasks" {
					ch <- prometheus.MustNewConstMetric(
						nodeCassandraRepairsActive,
						prometheus.GaugeValue,
						value,
						n.ID,
					)
				} else {
					log.Warnf("Unknown n::%s metric type %s", m.Name, m.Type)
				}

			case "clientRequestRead":
				if m.Type == "latency_per_operation" {
					ch <- prometheus.MustNewConstMetric(
						nodeClientRequestReadLatency,
						prometheus.GaugeValue,
						value*usTosecondsFactor,
						n.ID,
					)
				} else if m.Type == "95thPercentile" {
					ch <- prometheus.MustNewConstMetric(
						nodeClientRequestReadPercentile,
						prometheus.GaugeValue,
						value*usTosecondsFactor,
						n.ID,
					)
				} else {
					log.Warnf("Unknown n::%s metric type %s", m.Name, m.Type)
				}

			case "clientRequestWrite":
				if m.Type == "latency_per_operation" {
					ch <- prometheus.MustNewConstMetric(
						nodeClientRequestWriteLatency,
						prometheus.GaugeValue,
						value*usTosecondsFactor,
						n.ID,
					)
				} else if m.Type == "95thPercentile" {
					ch <- prometheus.MustNewConstMetric(
						nodeClientRequestWritePercentile,
						prometheus.GaugeValue,
						value*usTosecondsFactor,
						n.ID,
					)
				} else {
					log.Warnf("Unknown n::%s metric type %s", m.Name, m.Type)
				}
			}
		}
	}
}

// Describe describes all the metrics ever exported by the Instaclustr exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- clusterInfo
	ch <- clusterRunning
	ch <- clusterNodesCount
	ch <- clusterNodesRunningCount
	ch <- nodeInfo
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

// Collect fetches the stats from configured Instaclustr location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	clusters := []cluster{}
	dcs := new(datacentres)
	ms := []metrics{}
	wg := new(sync.WaitGroup)
	sem := &sync.Mutex{}

	// Fetching clusters list
	if err := json.Unmarshal(e.provisioningClient.GetClusters(), &clusters); err != nil {
		log.Errorf("Couldn't get clusters: %v", err)
		return
	}

	for _, c := range clusters {
		clusterInfoCollector(c, ch)
		clusterHealthCollector(c, ch)
		// Queryng status of the cluster, gathers the list of Datacentres
		if err := json.Unmarshal(e.provisioningClient.GetClusterStatus(c.ID), &dcs); err != nil {
			log.Errorf("Couldn't get cluster %s datacentres: %v", c.ID, err)
			return
		}
		for _, dc := range dcs.Dcs {
			for _, n := range dc.Nodes {
				wg.Add(1)
				go func(c cluster, n node, ch chan<- prometheus.Metric) {
					defer wg.Done()
					nodeInfoCollector(c, n, ch)
					nodeHealthCollector(c, n, ch)
					// Fetch all metrics from node
					sem.Lock()
					if err := json.Unmarshal(e.monitoringClient.GetNodeMetric(n.ID, strings.Join(allNodeMetricsQuery, ",")), &ms); err != nil {
						log.Errorf("Could not gather any metric: %v\n", err)
						return
					}
					sem.Unlock()
					// Collecting node metrics
					nodeMetricsCollector(c, n, ms, ch)

				}(c, n, ch)
			}
			// We don't close the channel, prometheus does the job
			wg.Wait()
		}
	}
}
