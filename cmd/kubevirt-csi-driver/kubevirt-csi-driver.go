package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/kubevirt/csi-driver/pkg/kubevirt"
	"github.com/kubevirt/csi-driver/pkg/service"
)

var (
	endpoint               = flag.String("endpoint", "unix:/csi/csi.sock", "CSI endpoint")
	namespace              = flag.String("namespace", "", "Namespace to run the controllers on")
	nodeName               = flag.String("node-name", "", "The node name - the node this pods runs on")
	infraClusterKubeconfig = flag.String("infra-cluster-kubeconfig", "", "Path to the infra cluster kubeconfig")
	infraClusterNamespace = flag.String("infra-cluster-namespace", "", "The namespace to operator on the infracluster")
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

	// TODO revise the assumption that the  current running node name should be the infracluster VM name.
	if *nodeName != "" {
		newClient, err := kubevirt.NewClient(c)
		if err != nil {
			klog.Fatal(fmt.Errorf("failed to create kubevirt client %v", err))
		}
		_, err = newClient.GetVMI(context.Background(), *infraClusterNamespace, *nodeName)
		if err != nil {
			klog.Fatal(fmt.Errorf("failed to find a VM in the infra cluster with that name %v: %v", nodeName, err))
		}
	}

	client, err := kubevirt.NewClient(c)
	if err != nil {
		klog.Fatal(err)
	}
	driver := service.NewkubevirtCSIDriver(*infraClusterClientSet, client, *nodeName)

	driver.Run(*endpoint)
}
