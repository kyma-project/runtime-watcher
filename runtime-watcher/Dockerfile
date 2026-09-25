FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

WORKDIR /app

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY main.go main.go
COPY pkg/ pkg/

# TAG_default_tag comes from image builder: https://github.com/kyma-project/test-infra/tree/main/cmd/image-builder
ARG TAG_default_tag=from_dockerfile
ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOFIPS140=v1.0.0 go build -ldflags="-X 'main.buildVersion=${TAG_default_tag}'" -a -o webhook main.go

FROM gcr.io/distroless/static:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6
WORKDIR /

COPY --from=builder /app/webhook .
USER 65532:65532

ENTRYPOINT ["/webhook"]
