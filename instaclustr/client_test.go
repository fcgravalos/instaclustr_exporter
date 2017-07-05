package instaclustr

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fcgravalos/instaclustr_exporter/mock"
)

func setup(up chan bool) {
	go func() {
		mock.NewMockServer()
	}()
	go func(chan bool) {
		mock.WaitForServerUp()
		up <- true
	}(up)
}

func TestGetClusters(t *testing.T) {
	clustersData := bytes.Trim(NewProvisioningClient().GetClusters(), "\n")
	expected := []byte(`[{"cassandraVersion":"apache-cassandra-2.1.10","derivedStatus":"RUNNING","id":"cluster-uuid-1","name":"MOCKED_CLUSTER_01","nodeCount":1,"runningNodeCount":1}]`)
	if !bytes.Equal(clustersData, expected) {
		t.Errorf("\nGetClusters returned unexpected data.\nGot:\n%sExpected:\n%s", string(clustersData), string(expected))
	}
}

func TestGetClusterStatus(t *testing.T) {
	cases := []struct {
		clusterID string
		expected  string
	}{
		{"cluster-uuid-1", `{"dataCentres":[{"cdcNetwork":{"network":"a.b.0.0","prefixLength":16},"encryptionKeyId":null,"id":"datacentre-uuid-1","name":"MOCKED_DATACENTRE_01","nodeCount":1,"nodes":[{"id":"node-uuid-1","nodeStatus":"RUNNING","privateAddress":"e.f.g.h","publicAddress":"a.b.c.d","rack":"MOCKED_RACK_01","size":"size","sparkJobserver":false,"sparkMaster":false,"zeppelin":false}],"provider":"AWS_VPC","resizeTargetNodeSize":null}]}`},
		{"unknown-cluster", `{"link":"https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html","message":"HTTP 404 Not Found","status":404}`},
	}
	for _, c := range cases {
		t.Logf("Testing GetClusterStatus with clusterID %s", c.clusterID)
		clusterStatus := bytes.Trim(NewProvisioningClient().GetClusterStatus(c.clusterID), "\n")
		expected := []byte(c.expected)
		if !bytes.Equal(clusterStatus, expected) {
			t.Errorf("GetClusterStatus returned unexpected data.\n- Got:\n%s\n- Expected:\n%s",
				string(clusterStatus),
				string(expected),
			)
		}
	}
}

func TestGetNodeMetric(t *testing.T) {
	allMetrics := strings.Join([]string{
		"n::cpuUtilization",     //Current CPU utilisation as a percentage of total available. Maximum value is 100%, regardless of the number of cores on the node.
		"n::diskUtilization",    //Total disk space utilisation, by Cassandra, as a percentage of total available.
		"n::cassandraReads",     //Reads per second by Cassandra.
		"n::cassandraWrites",    //Writes per second by Cassandra.
		"n::compactions",        //Number of pending compactions.
		"n::repairs",            //Number of active and pending repair tasks.
		"n::clientRequestRead",  //95th percentile distribution and average latency per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client).
		"n::clientRequestWrite", //95th percentile distribution and average latency per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client).
	}, ",")

	cases := []struct {
		nodeID   string
		metric   string
		expected string
	}{
		{"node-uuid-1", allMetrics,
			`[{"id":"node-uuid-1","payload":[{"metric":"clientRequestRead","type":"latency_per_operation","unit":"us/1","values":[{"time":"2017-07-03T09:37:04.000Z","value":"1462.5666666666664"}]},{"metric":"clientRequestRead","type":"95thPercentile","unit":"us","values":[{"time":"2017-07-03T09:37:04.000Z","value":"1866.1645999999998"}]},{"metric":"cpuUtilization","type":"percentage","unit":"1","values":[{"time":"2017-07-03T09:37:04.000Z","value":"2.5884383"}]},{"metric":"repairs","type":"activetasks","unit":"1","values":[{"time":"2017-07-03T09:37:04.000Z","value":"0.0"}]},{"metric":"repairs","type":"pendingtasks","unit":"1","values":[{"time":"2017-07-03T09:37:04.000Z","value":"0.0"}]},{"metric":"clientRequestWrite","type":"latency_per_operation","unit":"us/1","values":[{"time":"2017-07-03T09:37:04.000Z","value":"1293.5333333333335"}]},{"metric":"clientRequestWrite","type":"95thPercentile","unit":"us","values":[{"time":"2017-07-03T09:37:04.000Z","value":"1669.6253"}]},{"metric":"diskUtilization","type":"percentage","unit":"1","values":[{"time":"2017-07-03T09:37:04.000Z","value":"7.6197357"}]},{"metric":"cassandraReads","type":"count","unit":"1/s","values":[{"time":"2017-07-03T09:37:04.000Z","value":"1.25"}]},{"metric":"cassandraWrites","type":"count","unit":"1/s","values":[{"time":"2017-07-03T09:37:04.000Z","value":"1.25"}]},{"metric":"compactions","type":"pendingtasks","unit":"1","values":[{"time":"2017-07-03T09:37:04.000Z","value":"0.0"}]}]}]`},
		{"unknown-node", allMetrics, `{"link":"https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html","message":"HTTP 404 Not Found","status":404}`},
	}
	for _, c := range cases {
		t.Logf("Testing GetAllNodeMetrics with nodeID %s", c.nodeID)
		clusterStatus := bytes.Trim(NewMonitoringClient().GetNodeMetric(c.nodeID, c.metric), "\n")
		expected := []byte(c.expected)
		if !bytes.Equal(clusterStatus, expected) {
			t.Errorf("GetAllNodeMetrics returned unexpected data.\n- Got:\n%s\n- Expected:\n%s",
				string(clusterStatus),
				string(expected),
			)
		}
	}
}

func TestMain(m *testing.M) {
	up := make(chan bool)
	setup(up)
	<-up
	m.Run()
}
