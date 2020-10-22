# KubeVirt CSI driver

--- UNDER DEVELOPMENT ---

Implementation of a CSI driver for KubeVirt.


## Development

* `make build` 
* `make vet`
* `make fmt`
* `make vendor` to tidy and create vendor folder
* `make image` or to override `make image IMG=quay.io/rgolangh/kubevirt-csi-driver` (note: is uses podman. Symlink docker -> podman if you use docker)
