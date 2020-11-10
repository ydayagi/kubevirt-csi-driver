FROM registry.svc.ci.openshift.org/openshift/release:golang-1.15 AS builder

WORKDIR /src/kubevirt-csi-driver
COPY . .
RUN make build

FROM fedora:32

#RUN dnf install -y e2fsprogs xfsprogs
COPY --from=builder /src/kubevirt-csi-driver/bin/kubevirt-csi-driver .

ENTRYPOINT ["./kubevirt-csi-driver"]
