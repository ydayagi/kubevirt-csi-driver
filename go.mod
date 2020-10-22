module github.com/kubevirt/csi-driver

go 1.15

//require (
//	cloud.google.com/go v0.51.0 // indirect
//	github.com/Azure/go-autorest v11.1.2+incompatible // indirect
//	github.com/Azure/go-autorest/autorest v0.9.6 // indirect
//	github.com/blang/semver v3.5.1+incompatible // indirect
//	github.com/brancz/gojsontoyaml v0.0.0-20190425155809-e8bd32d46b3d // indirect
//	github.com/container-storage-interface/spec v1.2.0
//	github.com/coreos/bbolt v1.3.3 // indirect
//	github.com/coreos/etcd v3.3.17+incompatible // indirect
//	github.com/fortytw2/leaktest v1.3.0 // indirect
//	github.com/go-bindata/go-bindata v3.1.2+incompatible // indirect
//	github.com/golang/protobuf v1.4.2
//	github.com/hashicorp/go-version v1.1.0 // indirect
//	github.com/improbable-eng/thanos v0.3.2 // indirect
//	github.com/jsonnet-bundler/jsonnet-bundler v0.1.0 // indirect
//	github.com/kubernetes-csi/csi-lib-utils v0.7.0
//	github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348 // indirect
//	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452 // indirect
//	github.com/oklog/run v1.0.0 // indirect
//	github.com/onsi/ginkgo v1.12.1
//	github.com/onsi/gomega v1.10.1
//	github.com/openshift/client-go v0.0.0 // indirect
//	github.com/openshift/prom-label-proxy v0.1.1-0.20191016113035-b8153a7f39f1 // indirect
//	github.com/prometheus/tsdb v0.8.0 // indirect
//	github.com/spf13/pflag v1.0.5 // indirect
//	go.uber.org/zap v1.10.0 // indirect
//	golang.org/x/net v0.0.0-20200707034311-ab3426394381
//	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
//	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543 // indirect
//	google.golang.org/grpc v1.29.1
//	k8s.io/api v0.19.3
//	k8s.io/apimachinery v0.19.3
//	k8s.io/client-go v12.0.0+incompatible
//	k8s.io/klog v1.0.0
//	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
//	kubevirt.io/client-go v0.34.0
//	sigs.k8s.io/controller-runtime v0.6.2
//	sigs.k8s.io/controller-tools v0.2.4 // indirect
//	sigs.k8s.io/structured-merge-diff v1.0.1 // indirect
//	sigs.k8s.io/testing_frameworks v0.1.2 // indirect
//)
//
////replace github.com/openshift/client-go => github.com/openshift/client-go v0.0.0
//replace (
//	github.com/openshift/api => github.com/openshift/api v0.0.0-20191219222812-2987a591a72c
//	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20191125132246-f6563a70e19a
//	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a
//	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.4
//	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
//)

require (
	github.com/container-storage-interface/spec v1.3.0
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/golang/protobuf v1.4.3
	github.com/kubernetes-csi/csi-lib-utils v0.8.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	google.golang.org/grpc v1.33.1
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20201015054608-420da100c033
	sigs.k8s.io/controller-runtime v0.6.3
)
