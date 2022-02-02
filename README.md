# miab-dnsapi-proxy

miab-dnsapi-proxy proxies DNS API update requests to the [mailainabox](https://github.com/mail-in-a-box/mailinabox) DNS API.

Authorization takes place via a list of domains and host addresses allowed to update them for now.

## Container image

Get the container image from [Docker Hub](https://hub.docker.com/r/0xfelix/miab-dnsapi-proxy)

## TODO

- More elaborate authentication / authorization mechanism
- Add tests 

## Supported DNS APIs

API | Endpoint
---|---
lego HTTP request | POST `/httpreq/present`<br>POST `/httpreq/cleanup` (always returns `200 OK`)<br>(see https://go-acme.github.io/lego/dns/httpreq/)
ACMEDNS | POST `/acmedns/update`<br>(see https://github.com/joohoi/acme-dns#update-endpoint)
plain HTTP | GET `/plain/update` (query params `hostname` and `ip`)

## Environment variables

Variable | Type | Description | Required | Default
--- | --- | --- | --- | ---
`API_HOST` | string | Host address of mailinabox | Y |
`API_USER` | string | API user of mailinabox | Y |
`API_PASS` | string | Password of mailinabox API user | Y |
`API_TIMEOUT` | int | Timeout for calls to mailinabox API in seconds | N | 15 seconds
`ALLOWED_DOMAINS` | string | Combination of domains and CIDRs allowed to update them, example:<br>`example1.com,127.0.0.1;_acme-challenge.example2.com,127.0.0.1` | Y |
`LISTEN_ADDR` | string | Listen address of miab-dnsapi-proxy | N | `:8081`
`TRUSTED_PROXIES` | string | List of trusted proxy host addresses separated by comma | N | Trust all proxies
