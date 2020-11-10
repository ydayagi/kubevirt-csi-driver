package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"kubevirt.io/client-go/kubecli"

	certutil "k8s.io/client-go/util/cert"

	klog "github.com/sirupsen/logrus" //"k8s.io/klog"

	"github.com/kubevirt/csi-driver/pkg/kubevirt"
	"github.com/kubevirt/csi-driver/pkg/service"
)

var (
	endpoint              = flag.String("endpoint", "unix:/csi/csi.sock", "CSI endpoint")
	namespace             = flag.String("namespace", "", "Namespace to run the controllers on")
	nodeName              = flag.String("node-name", "", "The node name - the node this pods runs on")
	infraClusterNamespace = flag.String("infra-cluster-namespace", "", "The infra-cluster namespace")
	infraClusterApiUrl    = flag.String("infra-cluster-api-url", "", "The infra-cluster API URL")
	infraClusterToken     = flag.String("infra-cluster-token", "", "The infra-cluster token file")
	infraClusterCA        = flag.String("infra-cluster-ca", "", "the infra-cluster ca certificate file")
)

func init() {
	flag.Set("logtostderr", "true")
	//klog.InitFlags(flag.CommandLine)
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
	//klog.V(2).Infof("Driver vendor %v %v", service.VendorName, service.VendorVersion)
	klog.Infof("Driver vendor %v %v", service.VendorName, service.VendorVersion)

	tenantConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to build tenant cluster config: %v", err)
	}

	tenantClientSet, err := kubernetes.NewForConfig(tenantConfig)
	if err != nil {
		klog.Fatalf("Failed to build tenant client set: %v", err)
	}

	infraClusterConfig, err := buildInfraClusterConfig(*infraClusterApiUrl, *infraClusterToken, *infraClusterCA)
	if err != nil {
		//klog.V(2).Infof("Failed to build infra cluster config %v", err)
		klog.Infof("Failed to build infra cluster config %v", err)
	}

	infraClusterClient, err := dynamic.NewForConfig(infraClusterConfig)
	if err != nil {
		klog.Fatalf("Failed to initialize KubeVirt client %s", err)
	}

	virtClient, err := kubevirt.NewClient(infraClusterConfig)
	if err != nil {
		klog.Fatal(err)
	}

	var nodeId string
	if *nodeName != "" {
		node, err := tenantClientSet.CoreV1().Nodes().Get(*nodeName, v1.GetOptions{})
		if err != nil {
			klog.Fatal(fmt.Errorf("failed to find node by name %v: %v", nodeName, err))
		}

		// systemUUID is the VM ID
		nodeId = node.Status.NodeInfo.SystemUUID
	}

	kubevirtClient, err := kubecli.GetKubevirtClientFromRESTConfig(infraClusterConfig)
	if err != nil {
		klog.Fatal(err)
	}

	driver := service.NewkubevirtCSIDriver(virtClient, infraClusterClient, kubevirtClient, nodeId, *infraClusterNamespace)

	driver.Run(*endpoint)
}

func buildInfraClusterConfig(url string, tokenFile string, caFile string) (*rest.Config, error) {
	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}

	tlsClientConfig := rest.TLSClientConfig{}

	if _, err := certutil.NewPool(caFile); err != nil {
		klog.Errorf("Expected to load root CA config from %s, but got err: %v", caFile, err)
	} else {
		tlsClientConfig.CAFile = caFile
	}

	return &rest.Config{
		Host:            url,
		TLSClientConfig: tlsClientConfig,
		BearerToken:     string(token),
		BearerTokenFile: tokenFile,
	}, nil
}
