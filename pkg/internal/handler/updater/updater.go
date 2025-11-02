package updater

import (
	"net/http"

	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/config"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/internal/handler/updater/cloud"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/internal/handler/updater/dns"
)

func NewUpdater(cfg *config.Config) func(http.Handler) http.Handler {
	if cfg.CloudAPI {
		return cloud.NewUpdater(cfg)
	}

	return dns.NewUpdater(cfg)
}
