package driver

import (
	"net"
	"net/url"
	"os"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/seminar/nfs-csi-driver/pkg/metrics"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

const (
	DriverName    = "nfs.csi.seminar.dev"
	DriverVersion = "v0.1.0"
)

type Driver struct {
	nodeID      string
	nfsServer   string
	nfsBasePath string
	metricsPort int
	volumeSizes map[string]int64 // volumeID → 요청 용량
	mu          sync.Mutex
}

func New(nodeID, nfsServer, nfsBasePath string, metricsPort int) *Driver {
	return &Driver{
		nodeID:      nodeID,
		nfsServer:   nfsServer,
		nfsBasePath: nfsBasePath,
		metricsPort: metricsPort,
		volumeSizes: make(map[string]int64),
	}
}

func (d *Driver) Run(endpoint string) {
	go metrics.StartMetricsServer(d.metricsPort)

	u, err := url.Parse(endpoint)
	if err != nil {
		klog.Fatalf("Invalid endpoint: %v", err)
	}

	if u.Scheme == "unix" {
		if err := os.Remove(u.Path); err != nil && !os.IsNotExist(err) {
			klog.Fatalf("Failed to remove socket: %v", err)
		}
	}

	listener, err := net.Listen(u.Scheme, u.Path)
	if err != nil {
		klog.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	csi.RegisterIdentityServer(server, &IdentityServer{driver: d})
	csi.RegisterControllerServer(server, &ControllerServer{driver: d})
	csi.RegisterNodeServer(server, &NodeServer{driver: d})

	klog.Infof("CSI Driver listening on %s", endpoint)
	if err := server.Serve(listener); err != nil {
		klog.Fatalf("Failed to serve: %v", err)
	}
}
