package driver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/seminar/nfs-csi-driver/pkg/metrics"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

type NodeServer struct {
	csi.UnimplementedNodeServer
	driver *Driver
}

// Pod에 볼륨 마운트
func (n *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path is required")
	}

	volContext := req.GetVolumeContext()
	server := volContext["server"]
	path := volContext["path"]
	source := fmt.Sprintf("%s:%s", server, path)

	// 마운트 포인트 디렉토리 생성
	if err := os.MkdirAll(targetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create target path: %v", err)
	}

	// NFS 마운트 실행
	cmd := exec.Command("mount", "-t", "nfs", source, targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to mount NFS: %v, output: %s", err, out)
	}

	klog.Infof("NodePublishVolume: mounted %s → %s", source, targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

// Pod에서 볼륨 언마운트
func (n *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path is required")
	}

	cmd := exec.Command("umount", targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to umount: %v, output: %s", err, out)
	}

	klog.Infof("NodeUnpublishVolume: unmounted %s", targetPath)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// 볼륨 사용량 측정 → Prometheus 메트릭 업데이트 (차별점 기능!)
func (n *NodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volumePath := req.GetVolumePath()
	volumeID := req.GetVolumeId()

	var stat syscall.Statfs_t
	if err := syscall.Statfs(volumePath, &stat); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get volume stats: %v", err)
	}

	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bfree) * int64(stat.Bsize)
	used := total - free

	// Prometheus 메트릭 업데이트
	metrics.VolumeUsedBytes.WithLabelValues(volumeID).Set(float64(used))
	metrics.VolumeTotalBytes.WithLabelValues(volumeID).Set(float64(total))

	// 80% 임계치 초과 시 알림 메트릭 설정
	if total > 0 && float64(used)/float64(total) > 0.8 {
		metrics.VolumeUsageAlert.WithLabelValues(volumeID).Set(1)
		klog.Warningf("Volume %s usage exceeds 80%%: %d/%d bytes", volumeID, used, total)
	} else {
		metrics.VolumeUsageAlert.WithLabelValues(volumeID).Set(0)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Total:     total,
				Used:      used,
				Available: free,
				Unit:      csi.VolumeUsage_BYTES,
			},
		},
	}, nil
}

func (n *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},
		},
	}, nil
}

func (n *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: n.driver.nodeID,
	}, nil
}

func (n *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (n *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (n *NodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

var _ csi.NodeServer = &NodeServer{}
