package service

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	v1 "kubevirt.io/client-go/api/v1"

	"github.com/container-storage-interface/spec/lib/go/csi"

	client "github.com/kubevirt/csi-driver/pkg/kubevirt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	infraStorageClassNameParameter = "infraStorageClassName"
	busParameter                   = "bus"
	serialParameter                = "serial"
)

//ControllerService implements the controller interface
type ControllerService struct {
	infraClient           client.Client
	infraClusterNamespace string
}

var ControllerCaps = []csi.ControllerServiceCapability_RPC_Type{
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME, // attach/detach
}

// CreateVolume creates the disk for the request, unattached from any VM
func (c *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	log.Infof("Creating volume %s", req.Name)

	// Prepare parameters for the DataVolume
	storageClassName := req.Parameters[infraStorageClassNameParameter]
	volumeMode := getVolumeModeFromRequest(req)
	storageSize := req.GetCapacityRange().GetRequiredBytes()
	dvName := req.Name
	bus := req.Parameters[busParameter]

	// Create DataVolume object
	dv := &cdiv1.DataVolume{}
	dv.Name = dvName
	dv.Namespace = c.infraClusterNamespace
	dv.Kind = "DataVolume"
	dv.APIVersion = cdiv1.SchemeGroupVersion.String()
	dv.Spec.PVC = &corev1.PersistentVolumeClaimSpec{
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		StorageClassName: &storageClassName,
		VolumeMode:       &volumeMode,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: *resource.NewScaledQuantity(storageSize, 0)},
		},
	}
	dv.Spec.Source.Blank = &cdiv1.DataVolumeBlankImage{}

	// Create DataVolume
	dv, err := c.infraClient.CreateDataVolume(c.infraClusterNamespace, dv)

	if err != nil {
		log.Error("Failed creating DataVolume " + dvName)
		return nil, err
	}

	// Prepare serial for disk
	serial := string(dv.GetUID())

	// Return response
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: storageSize,
			VolumeId:      dvName,
			VolumeContext: map[string]string{
				busParameter:    bus,
				serialParameter: serial,
			},
		},
	}, nil
}

// DeleteVolume removes the data volume from kubevirt
func (c *ControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	dvName := req.VolumeId
	log.Infof("Removing data volume with %s", dvName)

	err := c.infraClient.DeleteDataVolume(c.infraClusterNamespace, dvName)
	if err != nil {
		log.Error("Failed deleting DataVolume " + dvName)
		return nil, err
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume takes a volume, which is an kubevirt disk, and attaches it to a node, which is an kubevirt VM.
func (c *ControllerService) ControllerPublishVolume(
	ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {

	dvName := req.VolumeId

	log.Infof("Attaching DataVolume %s to Node ID %s", dvName, req.NodeId)

	// Get VM name
	vmName, err := c.getVmNameByCSINodeID(req.NodeId)
	if err != nil {
		log.Error("Failed getting VM Name for node ID " + req.NodeId)
		return nil, err
	}

	// Determine disk name (disk-<DataVolume-name>)
	diskName := "disk-" + dvName

	// Determine serial number/string for the new disk
	serial := req.VolumeContext[serialParameter]

	// Determine BUS type
	bus := req.VolumeContext[busParameter]

	// hotplug DataVolume to VM
	log.Infof("Start attaching DataVolume %s to VM %s. Disk name: %s. Serial: %s. Bus: %s", dvName, vmName, diskName, serial, bus)

	hotplugRequest := &v1.HotplugVolumeRequest{
		Volume: &v1.Volume{
			VolumeSource: v1.VolumeSource{
				DataVolume: &v1.DataVolumeSource{
					Name: dvName,
				},
			},
			Name: diskName,
		},
		Disk: &v1.Disk{
			Name:   diskName,
			Serial: serial,
			DiskDevice: v1.DiskDevice{
				Disk: &v1.DiskTarget{
					Bus: bus,
				},
			},
		},
		Ephemeral: false,
	}
	err = c.infraClient.AddVolumeToVM(c.infraClusterNamespace, vmName, hotplugRequest)
	if err != nil {
		log.Error("Failed adding volume " + dvName + " to VM " + vmName)
		return nil, err
	}

	return &csi.ControllerPublishVolumeResponse{}, nil
}

// ControllerUnpublishVolume detaches the disk from the VM.
func (c *ControllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	dvName := req.VolumeId
	log.Infof("Detaching DataVolume %s from Node ID %s", dvName, req.NodeId)

	// Get VM name
	vmName, err := c.getVmNameByCSINodeID(req.NodeId)
	if err != nil {
		return nil, err
	}

	// Determine disk name (disk-<DataVolume-name>)
	diskName := "disk-" + dvName

	// Detach DataVolume from VM
	hotplugRequest := &v1.HotplugVolumeRequest{
		Volume: &v1.Volume{
			VolumeSource: v1.VolumeSource{
				DataVolume: &v1.DataVolumeSource{
					Name: dvName,
				},
			},
			Name: diskName,
		},
	}
	err = c.infraClient.RemoveVolumeFromVM(c.infraClusterNamespace, vmName, hotplugRequest)
	if err != nil {
		log.Error("Failed removing volume " + dvName + " from VM " + vmName)
		return nil, err
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

//ValidateVolumeCapabilities
func (c *ControllerService) ValidateVolumeCapabilities(context.Context, *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//ListVolumes
func (c *ControllerService) ListVolumes(context.Context, *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//GetCapacity
func (c *ControllerService) GetCapacity(context.Context, *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//CreateSnapshot
func (c *ControllerService) CreateSnapshot(context.Context, *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//DeleteSnapshot
func (c *ControllerService) DeleteSnapshot(context.Context, *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//ListSnapshots
func (c *ControllerService) ListSnapshots(context.Context, *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//ControllerExpandVolume
func (c *ControllerService) ControllerExpandVolume(context.Context, *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//ControllerGetCapabilities
func (c *ControllerService) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	caps := make([]*csi.ControllerServiceCapability, 0, len(ControllerCaps))
	for _, capability := range ControllerCaps {
		caps = append(
			caps,
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: capability,
					},
				},
			},
		)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (c *ControllerService) ControllerGetVolume(ctx context.Context, request *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {

	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: 0,
			VolumeId:      "TODO",
		},
	}, nil
}

// getVmNameByCSINodeID
// Find a VM in infra cluster by its firmware uuid. The uid is the ID that the CSI node
// part publishes in NodeGetInfo and then used by CSINode.spec.drivers[].nodeID
func (c *ControllerService) getVmNameByCSINodeID(nodeID string) (string, error) {
	list, err := c.infraClient.ListVirtualMachines(c.infraClusterNamespace)
	if err != nil {
		log.Error("Failed listing VMIs in infra cluster")
		return "", err
	}

	for _, vmi := range list {
		if strings.ToLower(string(vmi.Spec.Domain.Firmware.UUID)) == strings.ToLower(nodeID) {
			return vmi.Name, nil
		}
	}

	return "", fmt.Errorf("Failed to find VM with domain.firmware.uuid %v", nodeID)
}

func getVolumeModeFromRequest(req *csi.CreateVolumeRequest) corev1.PersistentVolumeMode {
	volumeMode := corev1.PersistentVolumeFilesystem // Set default in case not found in request

	for _, cap := range req.VolumeCapabilities {
		if cap == nil {
			continue
		}

		if cap.GetBlock() != nil {
			volumeMode = corev1.PersistentVolumeBlock
			break
		}
	}

	return volumeMode
}
