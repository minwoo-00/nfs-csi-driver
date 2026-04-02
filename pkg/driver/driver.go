package driver

import (
	"net"
	"net/url"
	"os"

	"github.com/seminar/nfs-csi-driver/pkg/metrics"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	"github.com/container-storage-interface/spec/lib/go/csi"
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
}

func New(nodeID, nfsServer, nfsBasePath string, metricsPort int) *Driver {
	return &Driver{
		nodeID:      nodeID,
		nfsServer:   nfsServer,
		nfsBasePath: nfsBasePath,
		metricsPort: metricsPort,
	}
}

func (d *Driver) Run(endpoint string) {
	// Prometheus 메트릭 서버 시작
	go metrics.StartMetricsServer(d.metricsPort)

	// Unix 소켓 파싱
	u, err := url.Parse(endpoint)
	if err != nil {
		klog.Fatalf("Invalid endpoint: %v", err)
	}

	// 기존 소켓 파일 제거
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

	// 세 가지 서비스 등록
	csi.RegisterIdentityServer(server, &IdentityServer{driver: d})
	csi.RegisterControllerServer(server, &ControllerServer{driver: d})
	csi.RegisterNodeServer(server, &NodeServer{driver: d})

	klog.Infof("CSI Driver listening on %s", endpoint)
	if err := server.Serve(listener); err != nil {
		klog.Fatalf("Failed to serve: %v", err)
	}
}