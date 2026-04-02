package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

type IdentityServer struct {
	csi.UnimplementedIdentityServer
	driver *Driver
}

// 드라이버 이름과 버전 반환
func (i *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          DriverName,
		VendorVersion: DriverVersion,
	}, nil
}

// 이 드라이버가 지원하는 기능 선언
func (i *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil
}

// 헬스체크
func (i *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}

// 인터페이스 구현 확인용 (컴파일 타임 체크)
var _ csi.IdentityServer = &IdentityServer{}
