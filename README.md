# InstaClustr Exporter [![Build Status](https://travis-ci.org/fcgravalos/instaclustr_exporter.svg?branch=master)](https://travis-ci.org/fcgravalos/instaclustr_exporter) [![CircleCI](https://circleci.com/gh/fcgravalos/instaclustr_exporter.svg?style=shield)](https://circleci.com/gh/fcgravalos/instaclustr_exporter)
Collects Cassandra metrics from InstaClustr Monitoring API and exports them to prometheus format.

To run it:

```bash
make
./instaclustr_exporter [flags]
```

## Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| cassandra_cluster_info | A mapping between the clusterId and clusterName  |clusterId, clusterName |
| cassandra_cluster_running | Whether or not the cassandra cluster is running |clusterId|
| cassandra_cluster_nodes_count| Number of nodes the cluster is composed|clusterId |
| cassandra_cluster_nodes_running_count |Number of nodes running in the cluster | clusterId|
| cassandra_node_info | A mapping between nodeId with its IPs, racks and cluster |clusterId, clusterName, nodeId, nodePublicIp, nodePrivateIp, rack|
| cassandra_node_running | Whether or not a single node is running |nodeId|
| cassandra_node_cpu_utilization_percentage | Current CPU utilisation as a percentage of total available. Maximum value is 100%, regardless of the number of cores on the node |nodeId|
| cassandra_node_disk_utilization_percentage | Total disk space utilisation, by Cassandra, as a percentage of total available |nodeId|
| cassandra_node_client_request_read_latency | Average latency (us/1) per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client) |nodeId|
| cassandra_node_client_request_write_latency | Average latency (us/1) per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client) |nodeId|
| cassandra_node_client_request_read_percentile | 95th percentile (us) distribution per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client) |nodeId|
| cassandra_node_client_request_write_percentile | 95th percentile (us) distribution per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client) |nodeId|
| cassandra_node_client_request_read_percentile99 | 99th percentile (us) distribution per client read request (i.e. the period from when a node receives a client request, gathers the records and response to the client) |nodeId|
| cassandra_node_client_request_write_percentile | 99th percentile (us) distribution per client write request (i.e. the period from when a node receives a client request, gathers the records and response to the client) |nodeId|
| cassandra_node_reads_per_second | Reads per second by Cassandra |nodeId|
| cassandra_node_writes_per_second | Writes per second by Cassandra |nodeId|
| cassandra_node_compactions | Number of pending compactions |nodeId|
| cassandra_node_repairs_active | Number of active repair tasks |nodeId|
| cassandra_node_repairs_pending | Number of pending repair tasks |nodeId|

### Flags

```bash
./instaclustr_exporter --help
```

* __`instaclustr.monitoring-apikey`:__
    Key for the provisioning API
* __`instaclustr.provisioning-apikey`:__
    Key for the provisioning API
* __`instaclustr.user`:__
    User for InstaClustr API
* __`log.format value`:__
    Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true" (default "logger:stderr")
* __`log.level value`:__
    Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
* __`version`:__
    Print version information.
* __`web.listen-address`:__
    Address to listen on for web interface and telemetry. (default ":9279")
* __`web.liveness-probe-url`:__
    URL for health-checks (default "/health")
* __`web.read-timeout`:__
    Read/Write Timeout (default 10s)
* __`web.shutdown-url`:__
    URL for health-checks (default "/shutdown")
* __`web.telemetry-path`:__
    Path under which to expose metrics. (default "/metrics")
* __`web.write-timeout`:__
    Read/Write Timeout (default 10s)

### Environment variables
* __`INSTACLUSTR_USER`:__
Takes precedence over __`instaclustr.user`__
* __`PROVISIONING_API_KEY`:__
Takes precedence over __`instaclustr.provisioning-apikey`__
* __`MONITORING_API_KEY`:__
Takes precedence over __`instaclustr.monitoring-apikey`__

## Using Docker

You can deploy this exporter using the [fcgravalos/instaclustr-exporter](https://registry.hub.docker.com/u/fcgravalos/instaclustr-exporter/) Docker image.

For example:

```bash
docker pull fcgravalos/instaclustr-exporter

docker run -d -p 9279:9279 fcgravalos/instaclustr-exporter \
 -instaclustr.user=user \
 -instaclustr.provisioning-apikey=myprovisioningkey \
 -instaclustr.monitoring-apikey=mymonitoringkey
```
