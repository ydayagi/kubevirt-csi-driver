package service

import (
	"k8s.io/klog"
	"kubevirt.io/client-go/kubecli"

	"k8s.io/client-go/dynamic"
)

var (
	// set by ldflags
	VendorVersion = "0.1.1"
	VendorName    = "csi.kubevirt.org"
)

type kubevirtCSIDriver struct {
	*IdentityService
	*ControllerService
	*NodeService
}

// NewkubevirtCSIDriver creates a driver instance
func NewkubevirtCSIDriver(infraClusterClient dynamic.Interface, kubevirtClient kubecli.KubevirtClient, tenantClusterClient dynamic.Interface, nodeId string, infraClusterNamespace string) *kubevirtCSIDriver {
	d := kubevirtCSIDriver{
		IdentityService:   &IdentityService{},
		ControllerService: &ControllerService{infraClusterClient, kubevirtClient, tenantClusterClient, infraClusterNamespace},
		NodeService:       &NodeService{nodeId: nodeId, kubevirtClient: kubevirtClient},
	}
	return &d
}

// Run will initiate the grpc services Identity, Controller, and Node.
func (driver *kubevirtCSIDriver) Run(endpoint string) {
	// run the gRPC server
	klog.Info("Setting the rpc server")

	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, driver.IdentityService, driver.ControllerService, driver.NodeService)
	s.Wait()
}
