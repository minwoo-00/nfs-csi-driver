package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/seminar/nfs-csi-driver/pkg/driver"
	"k8s.io/klog/v2"
)

var (
	endpoint   = flag.String("endpoint", "unix:///csi/csi.sock", "CSI endpoint")
	nodeID     = flag.String("nodeid", "", "Node ID")
	nfsServer  = flag.String("nfs-server", "", "NFS server address")
	nfsBasePath = flag.String("nfs-base-path", "/srv/nfs-data", "NFS base path on server")
	metricsPort = flag.Int("metrics-port", 9090, "Prometheus metrics port")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if *nodeID == "" {
		fmt.Fprintln(os.Stderr, "nodeid is required")
		os.Exit(1)
	}
	if *nfsServer == "" {
		fmt.Fprintln(os.Stderr, "nfs-server is required")
		os.Exit(1)
	}

	klog.Infof("Starting NFS CSI Driver - nodeID: %s, endpoint: %s", *nodeID, *endpoint)

	d := driver.New(*nodeID, *nfsServer, *nfsBasePath, *metricsPort)
	d.Run(*endpoint)
}