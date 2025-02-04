package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/onsi/gomega/gstruct"

	"github.com/0xfelix/hetzner-dnsapi-proxy/tests/libapi"
	"github.com/0xfelix/hetzner-dnsapi-proxy/tests/libserver"
)

var _ = Describe("AcmeDNS", func() {
	var (
		api    *ghttp.Server
		server *httptest.Server
		token  string
	)

	BeforeEach(func() {
		api = ghttp.NewServer()
		server, token = libserver.New(api.URL(), libapi.DefaultTTL)
	})

	AfterEach(func() {
		server.Close()
		api.Close()
	})

	Context("should succeed", func() {
		AfterEach(func() {
			Expect(api.ReceivedRequests()).To(HaveLen(3))
		})

		DescribeTable("creating a new record", func(ctx context.Context, subdomain string) {
			api.AppendHandlers(
				libapi.GetZones(token, libapi.Zones()),
				libapi.GetRecords(token, libapi.ZoneID, nil),
				libapi.PostRecord(token, libapi.NewTXTRecord()),
			)

			statusCode, resData := doAcmeDNSRequest(ctx, server.URL+"/acmedns/update", map[string]string{
				"subdomain": subdomain,
				"txt":       libapi.TXTUpdated,
			})
			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(resData).To(gstruct.MatchAllKeys(gstruct.Keys{
				"txt": Equal(libapi.TXTUpdated),
			}))
		},
			Entry("with prefix", libapi.TXTRecordNameFull),
			Entry("without prefix", libapi.TXTRecordNameNoPrefix),
		)

		DescribeTable("updating an existing record", func(ctx context.Context, subdomain string) {
			api.AppendHandlers(
				libapi.GetZones(token, libapi.Zones()),
				libapi.GetRecords(token, libapi.ZoneID, libapi.Records()),
				libapi.PutRecord(token, libapi.UpdatedTXTRecord()),
			)

			statusCode, resData := doAcmeDNSRequest(ctx, server.URL+"/acmedns/update", map[string]string{
				"subdomain": subdomain,
				"txt":       libapi.TXTUpdated,
			})
			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(resData).To(gstruct.MatchAllKeys(gstruct.Keys{
				"txt": Equal(libapi.TXTUpdated),
			}))
		},
			Entry("with prefix", libapi.TXTRecordNameFull),
			Entry("without prefix", libapi.TXTRecordNameNoPrefix),
		)
	})

	Context("should make no api calls and should fail", func() {
		AfterEach(func() {
			Expect(api.ReceivedRequests()).To(HaveLen(0))
		})

		It("when subdomain is missing", func(ctx context.Context) {
			statusCode, _ := doAcmeDNSRequest(ctx, server.URL+"/acmedns/update", map[string]string{
				"txt": libapi.TXTUpdated,
			})
			Expect(statusCode).To(Equal(http.StatusBadRequest))
		})

		It("when txt is missing", func(ctx context.Context) {
			statusCode, _ := doAcmeDNSRequest(ctx, server.URL+"/acmedns/update", map[string]string{
				"subdomain": libapi.TXTRecordNameFull,
			})
			Expect(statusCode).To(Equal(http.StatusBadRequest))
		})

		It("when subdomain is malformed", func(ctx context.Context) {
			statusCode, _ := doAcmeDNSRequest(ctx, server.URL+"/acmedns/update", map[string]string{
				"subdomain": libapi.TLD,
				"txt":       libapi.TXTUpdated,
			})
			Expect(statusCode).To(Equal(http.StatusBadRequest))
		})
	})
})

func doAcmeDNSRequest(ctx context.Context, url string, data map[string]string) (statusCode int, resData map[string]string) {
	body, err := json.Marshal(data)
	Expect(err).ToNot(HaveOccurred())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	Expect(err).ToNot(HaveOccurred())
	req.Header.Add("Content-Type", "application/json")

	c := &http.Client{}
	res, err := c.Do(req)
	Expect(err).ToNot(HaveOccurred())

	resBody, err := io.ReadAll(res.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Body.Close()).To(Succeed())

	if res.StatusCode == http.StatusOK {
		Expect(json.Unmarshal(resBody, &resData)).To(Succeed())
	} else {
		Expect(resBody).To(BeEmpty())
	}

	return res.StatusCode, resData
}
