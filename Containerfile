# Build binary

FROM golang:alpine as builder

RUN adduser --system --shell /bin/false miab-dnsapi-proxy

WORKDIR /workspace

COPY go.mod .
COPY go.sum .
COPY miab-dnsapi-proxy.go .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -tags timetzdata -tags=nomsgpack .

# Build image

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /workspace/miab-dnsapi-proxy /

USER miab-dnsapi-proxy
EXPOSE 8081
ENTRYPOINT ["/miab-dnsapi-proxy"]
