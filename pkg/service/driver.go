package service

import (
	"github.com/kubevirt/csi-driver/internal/kubevirt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// set by ldflags
	VendorVersion = "0.1.0"
	VendorName    = "csi.kubevirt.io"
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
func NewkubevirtCSIDriver(infraClusterClient kubernetes.Clientset, client kubevirt.Client, nodeId string) *kubevirtCSIDriver {
	d := kubevirtCSIDriver{
		IdentityService:    &IdentityService{},
		ControllerService:  &ControllerService{infraClusterClient: infraClusterClient},
		NodeService:        &NodeService{nodeId: nodeId, kubevirtClient: client},
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
