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
	VolumeUsedBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csi_volume_used_bytes",
		Help: "Used bytes of CSI volume",
	}, []string{"volume_id"})

	VolumeTotalBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csi_volume_total_bytes",
		Help: "Total bytes of CSI volume",
	}, []string{"volume_id"})

	VolumeUsageAlert = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csi_volume_usage_alert",
		Help: "1 if volume usage exceeds 80%",
	}, []string{"volume_id"})
)

func StartMetricsServer(port int) {
	http.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", port)
	klog.Infof("Metrics server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		klog.Errorf("Metrics server error: %v", err)
	}
}
