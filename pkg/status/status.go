package status

import (
	"net/http"

	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/data"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/key"

	"github.com/gin-gonic/gin"
)

func Ok(c *gin.Context) {
	c.Status(http.StatusOK)
}

func OkAcmeDns(c *gin.Context) {
	record := c.MustGet(key.RECORD).(*data.DnsRecord)
	c.JSON(http.StatusOK, gin.H{"txt": record.Value})
}
