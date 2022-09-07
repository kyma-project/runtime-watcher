# Build the manager binary
FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY kcp/go.mod go.mod
COPY kcp/go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY kcp/main.go main.go
COPY kcp/api/ api/
COPY kcp/controllers/ controllers/
COPY kcp/pkg/ pkg/
COPY skr/chart/skr-webhook/ chart/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/chart ./webhook-chart/
USER 65532:65532

ENTRYPOINT ["/manager"]
