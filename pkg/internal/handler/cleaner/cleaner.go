package cleaner

import (
	"net/http"

	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/config"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/internal/handler/cleaner/cloud"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/internal/handler/cleaner/dns"
)

func NewCleaner(cfg *config.Config) func(http.Handler) http.Handler {
	if cfg.CloudAPI {
		return cloud.NewCleaner(cfg)
	}

	return dns.NewCleaner(cfg)
}
