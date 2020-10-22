package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/kubevirt/csi-driver/internal/kubevirt"
	"github.com/kubevirt/csi-driver/pkg/service"
)

var (
	endpoint               = flag.String("endpoint", "unix:/csi/csi.sock", "CSI endpoint")
	namespace              = flag.String("namespace", "", "Namespace to run the controllers on")
	infraClusterKubeconfig = flag.String("infra-cluster-kubeconfig", "", "Path to the infra cluster kubeconfig")
	nodeName               = flag.String("node-name", "", "The node name - the node this pods runs on")
)

func init() {
	flag.Set("logtostderr", "true")
	klog.InitFlags(flag.CommandLine)
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	handle()
	os.Exit(0)
}

func handle() {
	if service.VendorVersion == "" {
		klog.Fatalf("VendorVersion must be set at compile time")
	}
	klog.V(2).Infof("Driver vendor %v %v", service.VendorName, service.VendorVersion)

	//get infra cluster client
	c, _ := clientcmd.BuildConfigFromFlags("", *infraClusterKubeconfig)
	infraClusterClientSet, err := kubernetes.NewForConfig(c)
	if err != nil {
		klog.Fatalf("Failed to initialize kubevirt client %s", err)
	}


	// get the node object by name and pass the VM ID because it is the node
	// id from the storage perspective. It will be used for attaching disks
	var nodeId string
	if *nodeName != "" {
		get, err := infraClusterClientSet.CoreV1().Nodes().Get(context.Background(), *nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Fatal(err)
		}
		nodeId = get.Status.NodeInfo.SystemUUID
	}

	client, err := kubevirt.NewClient(c)
	if err != nil {
		klog.Fatal(err)
	}
	driver := service.NewkubevirtCSIDriver(*infraClusterClientSet, client, nodeId)

	driver.Run(*endpoint)
}
