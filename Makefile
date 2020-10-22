# Image URL to use all building/pushing image targets
IMG ?= quay.io/kubevirt/csi-driver:latest

BINDIR=bin
#BINDATA=$(BINDIR)/go-bindata
BINDATA=go-bindata

REV=$(shell git describe --long --tags --match='v*' --always --dirty)

all: build

# Run tests
.PHONY: test
test:
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build the binary
.PHONY: build
build:
	go build -o $(BINDIR)/kubevirt-csi-driver -ldflags '-X version.Version=$(REV)' github.com/kubevirt/csi-driver/cmd/kubevirt-csi-driver

.PHONY: verify
verify: fmt vet

fmt:
	hack/verify-gofmt.sh
vet:
	hack/verify-govet.sh

.PHONY: image
image:
	podman build . -f Dockerfile -t ${IMG}

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify

$(BINDATA):
	go build -o $(BINDATA) ./vendor/github.com/jteeuwen/go-bindata/go-bindata
