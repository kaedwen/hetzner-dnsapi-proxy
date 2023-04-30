package status

import (
	"net/http"

	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/data"
	"github.com/gin-gonic/gin"
)

func Ok(c *gin.Context) {
	c.Status(http.StatusOK)
}

func OkAcmeDNS(c *gin.Context) {
	record := c.MustGet(data.KeyRecord).(*data.DNSRecord)
	c.JSON(http.StatusOK, gin.H{"txt": record.Value})
}
