FROM golang:1.22.6-alpine as builder

WORKDIR /app

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY main.go main.go
COPY internal/ internal/

# TAG_default_tag comes from image builder: https://github.com/kyma-project/test-infra/tree/main/cmd/image-builder
ARG TAG_default_tag=from_dockerfile

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.buildVersion=${TAG_default_tag}'" -a -o webhook main.go 

FROM gcr.io/distroless/static:nonroot
WORKDIR /

COPY --from=builder /app/webhook .
USER 65532:65532

ENTRYPOINT ["/webhook"]
