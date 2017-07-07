package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/fcgravalos/instaclustr_exporter/collector"
	"github.com/fcgravalos/instaclustr_exporter/common"
	"github.com/fcgravalos/instaclustr_exporter/instaclustr"
	"github.com/gorilla/mux"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<html>
					<head><title>InstaClustr Exporter</title></head>
					<body>
					<h1>InstaClustr Exporter</h1>
					<p><a href="/metrics">Metrics</a></p>
					</body>
					</html>`))
}

// NewExporter creates the InstaClustr Exporter
func NewExporter(telemetryPath string, serverOpts common.ServerOptions, instaclustrCfg instaclustr.Config) *common.Server {
	exp := collector.NewExporter(instaclustrCfg)
	prometheus.MustRegister(exp)
	// start httpServer
	s := common.NewServer("instaclustr_exporter", serverOpts)
	router := mux.NewRouter()
	router.HandleFunc("/", homeHandler).Methods("GET")
	router.HandleFunc(serverOpts.ShutdownURL, s.ShutDownHandler).Methods("GET")
	router.HandleFunc(serverOpts.LivenessProbeURL, s.LivenessProbeHandler).Methods("GET")
	router.Handle(telemetryPath, prometheus.Handler()).Methods("GET")
	s.HTTPServer.Handler = router
	return s
}

func main() {
	var (
		serverOpts     common.ServerOptions
		instaclustrCfg instaclustr.Config
		showVersion    = flag.Bool("version", false, "Print version information.")
		telemetryPath  = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	)

	flag.StringVar(&serverOpts.ListenAddress, "web.listen-address", ":9999", "Address to listen on for web interface and telemetry.")
	flag.StringVar(&serverOpts.LivenessProbeURL, "web.liveness-probe-url", "/health", "URL for health-checks")
	flag.StringVar(&serverOpts.ShutdownURL, "web.shutdown-url", "/shutdown", "URL for health-checks")
	flag.DurationVar(&serverOpts.ReadTimeOut, "web.read-timeout", 10*time.Second, "Read/Write Timeout")
	flag.DurationVar(&serverOpts.WriteTimeOut, "web.write-timeout", 10*time.Second, "Read/Write Timeout")
	flag.StringVar(&instaclustrCfg.User, "instaclustr.user", os.Getenv("INSTACLUSTR_USER"), "User for InstaClustr API")
	flag.StringVar(&instaclustrCfg.ProvisioningAPIKey, "instaclustr.provisioning-apikey", os.Getenv("PROVISIONING_API_KEY"), "Key for the provisioning API")
	flag.StringVar(&instaclustrCfg.MonitoringAPIKey, "instaclustr.monitoring-apikey", os.Getenv("MONITORING_API_KEY"), "Key for the provisioning API")

	flag.Parse()

	if *showVersion {
		fmt.Println(version.Print("instaclustr_exporter"))
		os.Exit(0)
	}
	s := NewExporter(*telemetryPath, serverOpts, instaclustrCfg)
	s.Start()
}
