package status

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/common"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/data"
)

func Ok(c *gin.Context) {
	c.Status(http.StatusOK)
}

func OkAcmeDNS(c *gin.Context) {
	record := c.MustGet(data.KeyRecord).(*data.DNSRecord)
	c.JSON(http.StatusOK, gin.H{"txt": record.Value})
}

func OkDirectAdmin(c *gin.Context) {
	common.StatusOkDirectAdmin(c)
}
