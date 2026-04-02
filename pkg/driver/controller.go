package driver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/seminar/nfs-csi-driver/pkg/metrics"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

type ControllerServer struct {
	csi.UnimplementedControllerServer
	driver *Driver
}

// PVC 생성 시 호출 → NFS 서버에 볼륨 디렉토리 생성
func (c *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name is required")
	}

	volumeID := req.GetName()
	volPath := filepath.Join(c.driver.nfsBasePath, volumeID)

	if err := os.MkdirAll(volPath, 0777); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create volume dir: %v", err)
	}

	// 요청 용량
	capacityBytes := req.GetCapacityRange().GetRequiredBytes()

	// capacity 파일 저장 (NodeGetVolumeStats에서 읽기 위해)
	capacityFile := filepath.Join(volPath, ".capacity")
	if err := os.WriteFile(capacityFile, []byte(fmt.Sprintf("%d", capacityBytes)), 0644); err != nil {
		klog.Warningf("Failed to write capacity file: %v", err)
	}

	// 오퍼레이션 카운터 증가
	metrics.VolumeOperationsTotal.WithLabelValues("create").Inc()
	// 활성 볼륨 수 증가
	metrics.VolumesTotal.Inc()

	klog.Infof("CreateVolume: created directory %s (capacity: %d bytes)", volPath, capacityBytes)

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeID,
			CapacityBytes: capacityBytes,
			VolumeContext: map[string]string{
				"server": c.driver.nfsServer,
				"path":   fmt.Sprintf("%s/%s", c.driver.nfsBasePath, volumeID),
			},
		},
	}, nil
}

// PVC 삭제 시 호출 → NFS 서버에서 볼륨 디렉토리 삭제
func (c *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	volPath := filepath.Join(c.driver.nfsBasePath, req.GetVolumeId())

	if err := os.RemoveAll(volPath); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to delete volume dir: %v", err)
	}

	// 오퍼레이션 카운터 증가
	metrics.VolumeOperationsTotal.WithLabelValues("delete").Inc()
	// 활성 볼륨 수 감소
	metrics.VolumesTotal.Dec()

	klog.Infof("DeleteVolume: removed directory %s", volPath)
	return &csi.DeleteVolumeResponse{}, nil
}

// accessMode 지원 여부 확인
func (c *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		},
	}, nil
}

func (c *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
		},
	}, nil
}
func (c *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (c *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

var _ csi.ControllerServer = &ControllerServer{}
