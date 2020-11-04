package service

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	"kubevirt.io/client-go/kubecli"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	nodeId             string
	infraClusterClient client.Client
	Client             client.Client
}

// NewkubevirtCSIDriver creates a driver instance
func NewkubevirtCSIDriver(infraClusterClient dynamic.Interface, kubevirtClient kubecli.KubevirtClient, tenantClusterClient dynamic.Interface, nodeId string) *kubevirtCSIDriver {
	d := kubevirtCSIDriver{
		IdentityService:   &IdentityService{},
		ControllerService: &ControllerService{infraClusterClient, kubevirtClient, tenantClustrClient},
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
