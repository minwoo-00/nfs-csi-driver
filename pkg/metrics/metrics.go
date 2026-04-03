package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

var (
	// PV별 사용량 (du 방식)
	VolumeUsedBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csi_volume_used_bytes",
		Help: "Used bytes of each CSI volume directory",
	}, []string{"volume_id", "pvc_name", "pvc_namespace"})

	// PV별 임계치 알림
	VolumeUsageAlert = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csi_volume_usage_alert",
		Help: "1 if volume usage exceeds 80% of NFS total",
	}, []string{"volume_id", "pvc_name", "pvc_namespace"})

	// NFS 서버 전체 현황
	NFSTotalBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "csi_nfs_total_bytes",
		Help: "Total bytes of NFS server disk",
	})

	NFSUsedBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "csi_nfs_used_bytes",
		Help: "Used bytes of NFS server disk",
	})

	NFSAvailableBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "csi_nfs_available_bytes",
		Help: "Available bytes of NFS server disk",
	})

	// 활성 볼륨 수
	VolumesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "csi_volumes_total",
		Help: "Total number of active CSI volumes",
	})

	// 오퍼레이션 카운터
	VolumeOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "csi_volume_operations_total",
		Help: "Total number of CSI volume operations",
	}, []string{"operation"})
)

func StartMetricsServer(port int) {
	http.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", port)
	klog.Infof("Metrics server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		klog.Errorf("Metrics server error: %v", err)
	}
}
