package service

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	v1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"

	"github.com/container-storage-interface/spec/lib/go/csi"

	klog "github.com/sirupsen/logrus" //"k8s.io/klog"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	ParameterThinProvisioning      = "thinProvisioning"
	infraStorageClassNameParameter = "infraStorageClassName"
	busParameter                   = "bus"
)

//ControllerService implements the controller interface
type ControllerService struct {
	infraClusterClient    dynamic.Interface
	kubevirtClient        kubecli.KubevirtClient
	infraClusterNamespace string
}

var ControllerCaps = []csi.ControllerServiceCapability_RPC_Type{
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME, // attach/detach
}

//CreateVolume creates the disk for the request, unattached from any VM
func (c *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.Infof("Creating volume %s", req.Name)

	// Create DataVolume object
	// Create DataVolume resource in infra cluster
	// Get details of new DataVolume resource
	// Wait until DataVolume is ready??
	dv := &cdiv1.DataVolume{}

	storageClassName := req.Parameters[infraStorageClassNameParameter]
	volumeMode := corev1.PersistentVolumeFilesystem // TODO: get it from req.VolumeCapabilities
	storageSize := req.GetCapacityRange().GetRequiredBytes()
	dv.Name = req.Name
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
	bus := req.Parameters[busParameter]

	resource := getDvGroupVersionResource()

	resultMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dv)
	if err != nil {
		klog.Error("Failed creating unstructured object from DataVolume object")
		return nil, err
	}

	unstructuredObj := &unstructured.Unstructured{}
	unstructuredObj.SetUnstructuredContent(resultMap)
	_, err = c.infraClusterClient.Resource(resource).Namespace(c.infraClusterNamespace).Create(unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		klog.Error("Failed creating DataVolume")
		return nil, err
	}

	unstructuredObj, err = c.infraClusterClient.Resource(resource).Namespace(c.infraClusterNamespace).Get(dv.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error("Failed getting DataVolume")
		return nil, err
	}

	// Return response
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: storageSize,
			VolumeId:      string(unstructuredObj.GetUID()),
			VolumeContext: map[string]string{busParameter: bus},
		},
	}, nil
}

//DeleteVolume removed the data volume from kubevirt
func (c *ControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.Infof("Removing data volume with ID %s", req.VolumeId)

	dvName, err := c.getDataVolumeNameByUID(ctx, req.VolumeId)
	if err != nil {
		return nil, err
	}

	err = c.infraClusterClient.Resource(getDvGroupVersionResource()).Namespace(c.infraClusterNamespace).Delete(dvName, &metav1.DeleteOptions{})
	return &csi.DeleteVolumeResponse{}, err
}

// ControllerPublishVolume takes a volume, which is an kubevirt disk, and attaches it to a node, which is an kubevirt VM.
func (c *ControllerService) ControllerPublishVolume(
	ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {

	klog.Infof("Attaching DataVolume UID %s to Node ID %s", req.VolumeId, req.NodeId)

	// Get DataVolume name by ID
	dvName, err := c.getDataVolumeNameByUID(ctx, req.VolumeId)
	if err != nil {
		return nil, err
	}

	// Get VM name
	vmName, err := c.getNodeNameByUID(ctx, req.NodeId)
	if err != nil {
		return nil, err
	}

	vmName = "ydayagi-vm-2-centos"

	// Determine disk name (disk-<DataVolume-name>)
	diskName := "disk-" + dvName

	// Determine serial number/string for the new disk
	serial := req.VolumeId[0:20]

	// Determine BUS type
	bus := req.VolumeContext[busParameter]

	// hotplug DataVolume to VM
	klog.Infof("Start attaching DataVolume %s to VM %s. Disk name: %s. Serial: %s. Bus: %s", dvName, vmName, diskName, serial, bus)

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
	err = c.kubevirtClient.VirtualMachine(c.infraClusterNamespace).AddVolume(vmName, hotplugRequest)
	if err != nil {
		return nil, err
	}

	return &csi.ControllerPublishVolumeResponse{}, nil
}

//ControllerUnpublishVolume detaches the disk from the VM.
func (c *ControllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	// req.NodeId == kubevirt VM name
	klog.Infof("Detaching DataVolume UID %s from Node ID %s", req.VolumeId, req.NodeId)

	// Get DataVolume name by ID
	dvName := req.VolumeId

	// Get VM name
	vmName, err := c.getNodeNameByUID(ctx, req.NodeId)
	if err != nil {
		return nil, err
	}

	vmName = "ydayagi-vm-2-centos"

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
	err = c.kubevirtClient.VirtualMachine(c.infraClusterNamespace).RemoveVolume(vmName, hotplugRequest)
	if err != nil {
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

func (c *ControllerService) getDataVolumeNameByUID(ctx context.Context, uid string) (string, error) {
	resource := getDvGroupVersionResource()

	list, err := c.infraClusterClient.Resource(resource).Namespace(c.infraClusterNamespace).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	dvName := ""

	for _, dv := range list.Items {
		if string(dv.GetUID()) == uid {
			dvName = dv.GetName()
			break
		}
	}

	if dvName == "" {
		return "", status.Error(codes.NotFound, "DataVolume uid: "+uid)
	}

	return dvName, nil
}

// getNodeNameByUID
// Assume that node name in tenant cluster is the same as VM resource name in infra cluster
func (c *ControllerService) getNodeNameByUID(ctx context.Context, uid string) (string, error) {
	resource := getNodesGroupVersionResource()

	list, err := c.infraClusterClient.Resource(resource).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	nodeName := ""

	for _, node := range list.Items {
		if string(node.GetUID()) == uid {
			nodeName = node.GetName()
			break
		}
	}

	if nodeName == "" {
		return "", status.Error(codes.NotFound, "Node uid: "+uid)
	}

	return nodeName, nil
}

func getNodesGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    cdiv1.SchemeGroupVersion.Group,
		Version:  cdiv1.SchemeGroupVersion.Version,
		Resource: "nodes",
	}
}

func getDvGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    cdiv1.SchemeGroupVersion.Group,
		Version:  cdiv1.SchemeGroupVersion.Version,
		Resource: "datavolumes",
	}
}
