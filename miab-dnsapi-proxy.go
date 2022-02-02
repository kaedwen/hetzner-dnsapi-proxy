package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/gin-gonic/gin"
)

type AllowedDomains map[string][]*net.IPNet

type Config struct {
	ApiHost        string         `env:"API_HOST"`
	ApiUser        string         `env:"API_USER,unset"`
	ApiPass        string         `env:"API_PASS,unset"`
	ApiTimeout     int            `env:"API_TIMEOUT" envDefault:"15"`
	AllowedDomains AllowedDomains `env:"ALLOWED_DOMAINS"`
	ListenAddr     string         `env:"LISTEN_ADDR" envDefault:":8081"`
	TrustedProxies []string       `env:"TRUSTED_PROXIES" envDefault:""`
}

type PlainData struct {
	Name  string `form:"hostname" binding:"required"`
	Value string `form:"ip" binding:"required"`
}

type AcmeDnsData struct {
	Name  string `json:"subdomain" binding:"required"`
	Value string `json:"txt" binding:"required"`
}

type HttpReqData struct {
	Name  string `json:"fqdn" binding:"required"`
	Value string `json:"value" binding:"required"`
}

type DnsRecordType int

const (
	A DnsRecordType = iota
	Txt
)

func (s DnsRecordType) String() string {
	switch s {
	case A:
		return "A"
	case Txt:
		return "TXT"
	}

	return ""
}

type DnsRecord struct {
	Name  string
	Value string
	Type  DnsRecordType
}

type DnsUpdateController struct {
	config     *Config
	apiMutex   *sync.Mutex
	httpClient *http.Client
}

const (
	KEY_DNS_RECORD           = "KEY_DNS_RECORD"
	KEY_BODY                 = "KEY_BODY"
	PREFIX_ACME_CHALLENGE    = "_acme-challenge."
	PREFIX_HTTPS             = "https://"
	API_SOMETHING_ISNT_RIGHT = "Something isn't right."
)

func (out *AllowedDomains) UnmarshalText(text []byte) error {
	allowedDomains := AllowedDomains{}

	textParts := strings.Split(string(text), ";")
	for _, textPart := range textParts {
		allowedDomainParts := strings.Split(textPart, ",")

		if len(allowedDomainParts) != 2 {
			return errors.New("failed to parse allowed domain, length of parts != 2")
		}

		_, ipv4Net, err := net.ParseCIDR(allowedDomainParts[1])
		if err != nil {
			return err
		}

		allowedDomains[allowedDomainParts[0]] = append(allowedDomains[allowedDomainParts[0]], ipv4Net)
	}

	*out = allowedDomains
	return nil
}

func forceHttpsToApiHost(config *Config) {
	re := regexp.MustCompile(`\w+://`)
	config.ApiHost = re.ReplaceAllLiteralString(config.ApiHost, "")
	config.ApiHost = PREFIX_HTTPS + config.ApiHost
}

func bindPlainData() gin.HandlerFunc {
	return func(c *gin.Context) {
		data := PlainData{}

		if err := c.BindQuery(&data); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		c.Set(KEY_DNS_RECORD, &DnsRecord{
			Name:  data.Name,
			Value: data.Value,
			Type:  A,
		})
	}
}

func bindAcmeDnsData() gin.HandlerFunc {
	return func(c *gin.Context) {
		data := AcmeDnsData{}

		if err := c.BindJSON(&data); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		c.Set(KEY_DNS_RECORD, &DnsRecord{
			Name:  PREFIX_ACME_CHALLENGE + data.Name,
			Value: data.Value,
			Type:  Txt,
		})
	}
}

func bindHttpReqData() gin.HandlerFunc {
	return func(c *gin.Context) {
		data := HttpReqData{}

		if err := c.BindJSON(&data); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		c.Set(KEY_DNS_RECORD, &DnsRecord{
			Name:  strings.TrimRight(data.Name, "."),
			Value: data.Value,
			Type:  Txt,
		})
	}
}

func isSubDomain(subDomain, wildcardDomain string) bool {
	// Domain must be a wildcard domain
	if wildcardDomain[0] != '*' {
		return false
	}

	wildcardDomainParts := strings.Split(wildcardDomain, ".")
	subDomainParts := strings.Split(subDomain, ".")

	// The subdomain must have at least the same amount of parts as the wildcard domain
	if len(subDomainParts) < len(wildcardDomainParts) {
		return false
	}

	// Up to the asterisk all domain parts must match
	subDomainPartsOffset := len(subDomainParts) - len(wildcardDomainParts)
	for i := len(wildcardDomainParts) - 1; i > 0; i-- {
		if wildcardDomainParts[i] != subDomainParts[i+subDomainPartsOffset] {
			return false
		}
	}

	return true
}

func (d *DnsUpdateController) checkPermissions() gin.HandlerFunc {
	return func(c *gin.Context) {
		updateAllowed := false
		dnsRecord := c.MustGet(KEY_DNS_RECORD).(*DnsRecord)

		for domain, ipNets := range d.config.AllowedDomains {
			if dnsRecord.Name != domain && !isSubDomain(dnsRecord.Name, domain) {
				continue
			}

			for _, ipNet := range ipNets {
				clientIp := net.ParseIP(c.ClientIP())
				if clientIp != nil && ipNet.Contains(clientIp) {
					updateAllowed = true
					break
				}
			}

			if updateAllowed {
				break
			}
		}

		if !updateAllowed {
			log.Printf("Client '%s' is not allowed to update '%s' record of '%s' to '%s'\n", c.ClientIP(), dnsRecord.Type, dnsRecord.Name, dnsRecord.Value)
			c.AbortWithStatus(http.StatusForbidden)
		}
	}
}

func (d *DnsUpdateController) getUrl(dnsRecord *DnsRecord) string {
	return fmt.Sprintf("%s/admin/dns/custom/%s/%s", d.config.ApiHost, dnsRecord.Name, dnsRecord.Type)
}

func (d *DnsUpdateController) doUpdate(dnsRecord *DnsRecord) (statusCode int, body string, err error) {
	req, err := http.NewRequest(http.MethodPut, d.getUrl(dnsRecord), strings.NewReader(dnsRecord.Value))
	if err != nil {
		return 0, "", err
	}
	req.SetBasicAuth(d.config.ApiUser, d.config.ApiPass)

	// mailinabox API can handle only one simultaneous request
	d.apiMutex.Lock()
	res, err := d.httpClient.Do(req)
	d.apiMutex.Unlock()
	if err != nil {
		return 0, "", err
	}

	defer res.Body.Close()
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, "", err
	}

	statusCode = res.StatusCode
	body = string(bodyBytes)

	if body == API_SOMETHING_ISNT_RIGHT {
		err = errors.New("mailinabox API error")
	}

	return
}

func (d *DnsUpdateController) updateDns() gin.HandlerFunc {
	return func(c *gin.Context) {
		dnsRecord := c.MustGet(KEY_DNS_RECORD).(*DnsRecord)

		log.Printf("Received request to update '%s' record of '%s' to '%s'\n", dnsRecord.Type, dnsRecord.Name, dnsRecord.Value)
		statusCode, body, err := d.doUpdate(dnsRecord)
		log.Printf("Update response status code '%d', body '%s'\n", statusCode, strings.TrimSpace(body))

		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if statusCode != http.StatusOK {
			c.AbortWithStatus(statusCode)
			return
		}

		c.Set(KEY_BODY, body)
	}
}

func NewDnsUpdateController(config *Config) *DnsUpdateController {
	return &DnsUpdateController{
		config,
		&sync.Mutex{},
		&http.Client{
			Timeout: time.Duration(config.ApiTimeout) * time.Second,
		},
	}
}

func statusOkBody(c *gin.Context) {
	c.String(http.StatusOK, c.GetString(KEY_BODY))
}

func statusOkAcmeDns(c *gin.Context) {
	dnsRecord := c.MustGet(KEY_DNS_RECORD).(*DnsRecord)
	c.JSON(http.StatusOK, gin.H{"txt": dnsRecord.Value})
}

func startServer(listenAddr string, r *gin.Engine) {
	s := &http.Server{
		Addr:    listenAddr,
		Handler: r,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down miab-dnsapi-proxy")

	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(c); err != nil {
		log.Fatal("Forcing shutdown:", err)
	}
}

func main() {
	config := &Config{}
	if err := env.Parse(config, env.Options{RequiredIfNoDef: true}); err != nil {
		log.Fatal(err)
	}

	forceHttpsToApiHost(config)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	if len(config.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(config.TrustedProxies); err != nil {
			log.Fatal(err)
		}
	}

	d := NewDnsUpdateController(config)
	r.GET("/plain/update", bindPlainData(), d.checkPermissions(), d.updateDns(), statusOkBody)
	r.POST("/acmedns/update", bindAcmeDnsData(), d.checkPermissions(), d.updateDns(), statusOkAcmeDns)
	r.POST("/httpreq/present", bindHttpReqData(), d.checkPermissions(), d.updateDns(), statusOkBody)
	r.POST("/httpreq/cleanup", statusOkBody)

	log.Printf("Starting miab-dnsapi-proxy, listening on %s\n", config.ListenAddr)
	startServer(config.ListenAddr, r)
}
