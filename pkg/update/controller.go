package update

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/config"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/data"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/hetzner"
	"github.com/0xfelix/hetzner-dnsapi-proxy/pkg/key"

	"github.com/gin-gonic/gin"
)

const (
	baseUrl = "https://dns.hetzner.com/api/v1"
)

type Controller struct {
	cfg    *config.Config
	mutex  *sync.Mutex
	client *http.Client
}

func NewController(cfg *config.Config) *Controller {
	return &Controller{
		cfg,
		&sync.Mutex{},
		&http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

func (d *Controller) CheckPermissions() gin.HandlerFunc {
	return func(c *gin.Context) {
		record := c.MustGet(key.RECORD).(*data.DnsRecord)

		for domain, ipNets := range d.cfg.AllowedDomains {
			if record.FullName != domain && !isSubDomain(record.FullName, domain) {
				continue
			}

			for _, ipNet := range ipNets {
				ip := net.ParseIP(c.ClientIP())
				if ip != nil && ipNet.Contains(ip) {
					return
				}
			}
		}

		log.Printf("Client '%s' is not allowed to update '%s' data of '%s' to '%s'\n", c.ClientIP(), record.Type, record.FullName, record.Value)
		c.AbortWithStatus(http.StatusForbidden)
	}
}

func (d *Controller) UpdateDns() gin.HandlerFunc {
	return func(c *gin.Context) {
		record := c.MustGet(key.RECORD).(*data.DnsRecord)
		log.Printf("Received request to update '%s' data of '%s' to '%s'\n", record.Type, record.FullName, record.Value)

		if err := d.do(record); err != nil {
			log.Printf("Update failed: %v", err)
			_ = c.AbortWithError(http.StatusInternalServerError, err)
		}
	}
}

func (d *Controller) do(record *data.DnsRecord) error {
	// Ensure only one simultaneous update sequence
	d.mutex.Lock()
	defer d.mutex.Unlock()

	zIds, err := d.getZoneIds()
	if err != nil {
		return err
	}

	zId := zIds[record.Zone]
	if zId == "" {
		return fmt.Errorf("could not find zone id for record %s", record.FullName)
	}

	rIds, err := d.getRecordIds(zId, record.Type)
	if err != nil {
		return err
	}

	r := hetzner.Record{
		Name:   record.Name,
		TTL:    d.cfg.RecordTTL,
		Type:   record.Type,
		Value:  record.Value,
		ZoneId: zId,
	}

	if rId, ok := rIds[record.Name]; ok {
		r.Id = rId
		return d.updateRecord(&r)
	}

	return d.createRecord(&r)
}

func (d *Controller) getRequest(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Auth-API-Token", d.cfg.Token)

	res, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get request failed with statuscode %d", res.StatusCode)
	}

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return resBody, nil
}

func (d *Controller) jsonRequest(method, url string, body []byte) error {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Auth-API-Token", d.cfg.Token)

	res, err := d.client.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s request failed with statuscode %d", method, res.StatusCode)
	}

	return nil
}

func (d *Controller) getZoneIds() (map[string]string, error) {
	res, err := d.getRequest(baseUrl + "/zones")
	if err != nil {
		return nil, nil
	}

	z := hetzner.Zones{}
	if err := json.Unmarshal(res, &z); err != nil {
		return nil, nil
	}

	ids := map[string]string{}
	for _, zone := range z.Zones {
		ids[zone.Name] = zone.Id
	}

	return ids, nil
}

func (d *Controller) getRecordIds(zoneId, recordType string) (map[string]string, error) {
	res, err := d.getRequest(baseUrl + "/records?zone_id=" + zoneId)
	if err != nil {
		return nil, nil
	}

	r := hetzner.Records{}
	if err := json.Unmarshal(res, &r); err != nil {
		return nil, nil
	}

	ids := map[string]string{}
	for _, record := range r.Records {
		if record.Type == recordType {
			ids[record.Name] = record.Id
		}
	}

	return ids, nil
}

func (d *Controller) createRecord(record *hetzner.Record) error {
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return d.jsonRequest(http.MethodPost, baseUrl+"/records", body)
}

func (d *Controller) updateRecord(record *hetzner.Record) error {
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return d.jsonRequest(http.MethodPut, baseUrl+"/records/"+record.Id, body)
}
