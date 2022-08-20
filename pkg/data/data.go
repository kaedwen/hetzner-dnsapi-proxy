package data

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/key"

	"github.com/gin-gonic/gin"
)

const (
	prefixAcmeChallenge = "_acme-challenge"
	recordTypeA         = "A"
	recordTypeTxt       = "TXT"
)

type DnsRecord struct {
	FullName string
	Name     string
	Zone     string
	Value    string
	Type     string
}

type plainData struct {
	FullName string `form:"hostname" binding:"required"`
	Value    string `form:"ip" binding:"required"`
}

type acmeDnsData struct {
	FullName string `json:"subdomain" binding:"required"`
	Value    string `json:"txt" binding:"required"`
}

type httpReqData struct {
	FullName string `json:"fqdn" binding:"required"`
	Value    string `json:"value" binding:"required"`
}

func BindPlain() gin.HandlerFunc {
	return func(c *gin.Context) {
		data := plainData{}

		if err := c.BindQuery(&data); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		name, zone := splitFullName(data.FullName)
		c.Set(key.RECORD, &DnsRecord{
			FullName: data.FullName,
			Name:     name,
			Zone:     zone,
			Value:    data.Value,
			Type:     recordTypeA,
		})
	}
}

func BindAcmeDns() gin.HandlerFunc {
	return func(c *gin.Context) {
		data := acmeDnsData{}

		if err := c.Bind(&data); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		// prepand prefix if not already given
		if !strings.HasPrefix(data.FullName, prefixAcmeChallenge) {
			data.FullName = fmt.Sprintf("%s.%s", prefixAcmeChallenge, data.FullName)
		}

		name, zone := splitFullName(data.FullName)
		c.Set(key.RECORD, &DnsRecord{
			FullName: data.FullName,
			Name:     name,
			Zone:     zone,
			Value:    data.Value,
			Type:     recordTypeTxt,
		})
	}
}

func BindHttpReq() gin.HandlerFunc {
	return func(c *gin.Context) {
		data := httpReqData{}

		if err := c.BindJSON(&data); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		data.FullName = strings.TrimRight(data.FullName, ".")
		name, zone := splitFullName(data.FullName)
		c.Set(key.RECORD, &DnsRecord{
			FullName: data.FullName,
			Name:     name,
			Zone:     zone,
			Value:    data.Value,
			Type:     recordTypeTxt,
		})
	}
}

func splitFullName(n string) (name string, zone string) {
	parts := strings.Split(n, ".")
	length := len(parts)

	for i := 0; i < length-2; i++ {
		name += parts[i]
		if i < length-3 {
			name += "."
		}
	}

	zone = fmt.Sprintf("%s.%s", parts[length-2], parts[length-1])

	return
}
