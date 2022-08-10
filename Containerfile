# Build binary

FROM golang:alpine as builder

RUN apk add --update make

RUN adduser --system --shell /bin/false hetzner-dnsapi-proxy

WORKDIR /workspace

COPY Makefile .
COPY go.mod .
COPY go.sum .
COPY main.go .
COPY pkg/ .

RUN go mod download

RUN make build

# Build image

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /workspace/hetzner-dnsapi-proxy /

USER hetzner-dnsapi-proxy
EXPOSE 8081
ENTRYPOINT ["/hetzner-dnsapi-proxy"]
