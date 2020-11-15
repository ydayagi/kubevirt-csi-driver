package service

import (
	klog "github.com/sirupsen/logrus" //"k8s.io/klog"
	"kubevirt.io/client-go/kubecli"

	"github.com/kubevirt/csi-driver/pkg/kubevirt"
	"k8s.io/client-go/dynamic"
)

var (
	// set by ldflags
	VendorVersion = "0.1.1"
	VendorName    = "csi.kubevirt.io"
)

type kubevirtCSIDriver struct {
	*IdentityService
	*ControllerService
	*NodeService
}

// NewkubevirtCSIDriver creates a driver instance
func NewkubevirtCSIDriver(internalInfraClient kubevirt.Client, infraClusterClient dynamic.Interface, kubevirtClient kubecli.KubevirtClient, nodeId string, infraClusterNamespace string) *kubevirtCSIDriver {
	d := kubevirtCSIDriver{
		IdentityService:   &IdentityService{internalInfraClient},
		ControllerService: &ControllerService{internalInfraClient, infraClusterNamespace},
		NodeService:       &NodeService{nodeId: nodeId, infraClusterClient: infraClusterClient, kubevirtClient: kubevirtClient},
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
