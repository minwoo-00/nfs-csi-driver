package driver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	if err := os.MkdirAll(targetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create target path: %v", err)
	}

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

// 볼륨 사용량 측정 → Prometheus 메트릭 업데이트
func (n *NodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volumePath := req.GetVolumePath()
	volumeID := req.GetVolumeId()

	// 1. PV 디렉토리 실제 사용량 (du 방식)
	out, err := exec.Command("du", "-sb", volumePath).Output()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get volume usage: %v", err)
	}
	var used int64
	fmt.Sscanf(string(out), "%d", &used)

	// 2. NFS 서버 전체 통계 (Statfs 방식)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(volumePath, &stat); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get fs stats: %v", err)
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bfree) * int64(stat.Bsize)

	// 3. capacity 파일에서 용량 + PVC 정보 읽기
	var capacityBytes int64
	var pvcName, pvcNamespace string
	capacityFile := filepath.Join(volumePath, ".capacity")
	if data, err := os.ReadFile(capacityFile); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) >= 1 {
			fmt.Sscanf(lines[0], "%d", &capacityBytes)
		}
		if len(lines) >= 2 {
			pvcName = strings.TrimSpace(lines[1])
		}
		if len(lines) >= 3 {
			pvcNamespace = strings.TrimSpace(lines[2])
		}
	} else {
		klog.Warningf("Failed to read capacity file: %v", err)
	}

	// 4. Prometheus 메트릭 업데이트
	metrics.VolumeUsedBytes.WithLabelValues(volumeID, pvcName, pvcNamespace).Set(float64(used))
	metrics.NFSTotalBytes.Set(float64(total))
	metrics.NFSUsedBytes.Set(float64(total - free))
	metrics.NFSAvailableBytes.Set(float64(free))

	// 5. PVC 요청 용량 기준 80% 임계치 체크
	if capacityBytes > 0 && float64(used)/float64(capacityBytes) > 0.8 {
		metrics.VolumeUsageAlert.WithLabelValues(volumeID, pvcName, pvcNamespace).Set(1)
		klog.Warningf("Volume %s(%s/%s) usage exceeds 80%%: %d/%d bytes",
			volumeID, pvcNamespace, pvcName, used, capacityBytes)
	} else {
		metrics.VolumeUsageAlert.WithLabelValues(volumeID, pvcName, pvcNamespace).Set(0)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Total:     capacityBytes,
				Used:      used,
				Available: capacityBytes - used,
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
