package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/mount"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/kubevirt/csi-driver/internal/kubevirt"
	//kubevirtv1 "kubevirt.io/client-go/api/v1"
	"golang.org/x/net/context"
	"k8s.io/klog"
)

type NodeService struct {
	nodeId             string
	infraClusterClient kubernetes.Clientset
	kubevirtClient     kubevirt.Client
}

var NodeCaps = []csi.NodeServiceCapability_RPC_Type{
	csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
}

// NodeStageVolume prepares the volume for usage. If it's an FS type it creates a file system on the volume.
func (n *NodeService) NodeStageVolume(_ context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.Infof("Staging volume %s with %+v", req.VolumeId, req)

	// get the VMI volumes which are under VMI.spec.volumes
	// The volume ID to prepare should

	device, err := getDeviceBySerialID(req.VolumeId)
	if err != nil {
		klog.Errorf("Failed to fetch device by attachment-id for volume %s on node %s", req.VolumeId, n.nodeId)
		return nil, err
	}

	// is there a filesystem on this device?
	//filesystem, err := getDeviceInfo(device)
	if device.Fstype != "" {
		klog.Infof("Detected fs %s, returning", device.Fstype)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	fsType := req.VolumeCapability.GetMount().FsType
	// no filesystem - create it
	klog.Infof("Creating FS %s on device %s", fsType, device)
	err = makeFS(device.Path, fsType)
	if err != nil {
		klog.Errorf("Could not create filesystem %s on %s", fsType, device)
		return nil, err
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (n *NodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (n *NodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// volumeID is kubevirt's serialID
	// TODO link to kubevirt code
	device, err := getDeviceBySerialID(req.VolumeId)
	if err != nil {
		klog.Errorf("Failed to fetch device by attachment-id for volume %s on node %s", req.VolumeId, n.nodeId)
		return nil, err
	}

	targetPath := req.GetTargetPath()
	err = os.MkdirAll(targetPath, 0750)
	if err != nil {
		return nil, err
	}

	fsType := req.VolumeCapability.GetMount().FsType
	klog.Infof("Mounting devicePath %s, on targetPath: %s with FS type: %s",
		device, targetPath, fsType)
	mounter := mount.New("")
	err = mounter.Mount(device.Path, targetPath, fsType, []string{})
	if err != nil {
		klog.Errorf("Failed mounting %v", err)
		return nil, err
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (n *NodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	mounter := mount.New("")
	klog.Infof("Unmounting %s", req.GetTargetPath())
	err := mounter.Unmount(req.GetTargetPath())
	if err != nil {
		klog.Infof("Failed to unmount")
		return nil, err
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (n *NodeService) NodeGetVolumeStats(context.Context, *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	panic("implement me")
}

func (n *NodeService) NodeExpandVolume(context.Context, *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	panic("implement me")
}

func (n *NodeService) NodeGetInfo(context.Context, *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{NodeId: n.nodeId}, nil
}

func (n *NodeService) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	caps := make([]*csi.NodeServiceCapability, 0, len(NodeCaps))
	for _, c := range NodeCaps {
		caps = append(
			caps,
			&csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: c,
					},
				},
			},
		)
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

type Devices struct {
	BlockDevices []Device `json:"blockdevices"`
}
type Device struct {
	SerialID string `json:"serial"`
	Path     string `json:"path"`
	Fstype   string `json:"fstype"`
}

func getDeviceBySerialID(serialID string) (Device, error) {
	klog.Infof("Get the device details by serialID %s", serialID)
	klog.V(5).Info("lsblk -nJo SERIAL,PATH,FSTYPE")
	// must be lsblk recent enough for json format
	cmd := exec.Command("lsblk", "-nJo", "SERIAL,PATH,FSTYPE")
	out, err := cmd.Output()
	exitError, incompleteCmd := err.(*exec.ExitError)
	if err != nil && incompleteCmd {
		return Device{}, errors.New(err.Error() + "lsblk failed with " + string(exitError.Stderr))
	}

	devices := Devices{}
	err = json.Unmarshal(out, &devices)
	if err != nil {
		klog.Errorf("Failed to parse json output from lsblk: %s", err)
		return Device{}, err
	}

	for _, d := range devices.BlockDevices {
		if d.SerialID == serialID {
			return d, nil
		}
	}
	return Device{}, errors.New("couldn't find device by serial id")
}

// getDeviceInfo will return the first Device which is a partition and its filesystem.
// if the given Device disk has no partition then an empty zero valued device will return
func getDeviceInfo(device string) (string, error) {
	devicePath, err := filepath.EvalSymlinks(device)
	if err != nil {
		klog.Errorf("Unable to evaluate symlink for device %s", device)
		return "", errors.New(err.Error())
	}

	klog.Info("lsblk -nro FSTYPE ", devicePath)
	cmd := exec.Command("lsblk", "-nro", "FSTYPE", devicePath)
	out, err := cmd.Output()
	exitError, incompleteCmd := err.(*exec.ExitError)
	if err != nil && incompleteCmd {
		return "", errors.New(err.Error() + "lsblk failed with " + string(exitError.Stderr))
	}

	reader := bufio.NewReader(bytes.NewReader(out))
	line, _, err := reader.ReadLine()
	if err != nil {
		klog.Errorf("Error occured while trying to read lsblk output")
		return "", err
	}
	return string(line), nil
}

func makeFS(device string, fsType string) error {
	// caution, use force flag when creating the filesystem if it doesn't exit.
	klog.Infof("Mounting device %s, with FS %s", device, fsType)

	var cmd *exec.Cmd
	var stdout, stderr bytes.Buffer
	if strings.HasPrefix(fsType, "ext") {
		cmd = exec.Command("mkfs", "-F", "-t", fsType, device)
	} else if strings.HasPrefix(fsType, "xfs") {
		cmd = exec.Command("mkfs", "-t", fsType, "-f", device)
	} else {
		return errors.New(fsType + " is not supported, only xfs and ext are supported")
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitError, incompleteCmd := err.(*exec.ExitError)
	if err != nil && incompleteCmd {
		klog.Errorf("stdout: %s", string(stdout.Bytes()))
		klog.Errorf("stderr: %s", string(stderr.Bytes()))
		return errors.New(err.Error() + " mkfs failed with " + string(exitError.Error()))
	}

	return nil
}

// isMountpoint find out if a given directory is a real mount point
func isMountpoint(mountDir string) bool {
	cmd := exec.Command("findmnt", mountDir)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false
	}
	return true
}

func baseDevicePathByInterface(diskInterface string) (string, error) {

	//TODO replace this non-sense with lsblk  -o SERIAL,PATH -J which creates
	// json representaion of block devices serial and path
	// {
	// "blockdevices": [
	//    {"serial":"S35ENX0J663758", "path":"/dev/nvme0n1"},
	// ]

	switch diskInterface {
	case "virtio":
		return "/dev/disk/by-id/virtio-", nil
	case "scsi":
		return "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_", nil
	}
	return "", errors.New("device type is unsupported")

}
