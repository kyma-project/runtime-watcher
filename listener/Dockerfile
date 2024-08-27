## Run from root context!!
FROM golang:1.22.6-alpine as builder

WORKDIR /app

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY pkg/ pkg/
COPY example-mtls/kcp/listener/main.go main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o listener main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /

COPY --from=builder /app/listener .
USER 65532:65532

EXPOSE 8089/tcp

ENTRYPOINT ["/listener"]
