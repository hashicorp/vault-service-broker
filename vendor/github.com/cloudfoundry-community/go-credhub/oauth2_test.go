package credhub_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	credhub "github.com/cloudfoundry-community/go-credhub"
	. "github.com/onsi/gomega"
)

func TestUAAEndpoint(t *testing.T) {
	spec.Run(t, "UAAEndpoint", testUAAEndpoint, spec.Report(report.Terminal{}))
}

func testUAAEndpoint(t *testing.T, when spec.G, it spec.S) {
	var (
		server *httptest.Server
	)

	it.Before(func() {
		RegisterTestingT(t)
		server = mockCredhubServer()
	})

	it.After(func() {
		server.Close()
	})

	when("Getting the UAA endpoint", func() {
		it("works", func() {
			endpoint, err := credhub.UAAEndpoint(server.URL, true)
			Expect(err).NotTo(HaveOccurred())

			authURL := endpoint.AuthURL
			tokenURL := endpoint.TokenURL

			Expect(authURL).NotTo(BeZero())
			Expect(tokenURL).NotTo(BeZero())

			idx := strings.Index(authURL, "/oauth")
			Expect(idx).To(BeNumerically(">", 0))
			baseAuthURL := authURL[:idx]

			Expect(baseAuthURL).NotTo(Equal(server.URL))

			baseTokenURL := tokenURL[:idx]
			Expect(baseAuthURL).To(Equal(baseTokenURL))
		})
	})

	when("the server url is invalid", func() {
		it("fails", func() {
			endpoint, err := credhub.UAAEndpoint("badscheme://bad_host\\", false)
			Expect(err).To(HaveOccurred())
			Expect(endpoint).To(BeZero())
		})
	})

	when("the server returns invalid json", func() {
		it("fails", func() {
			endpoint, err := credhub.UAAEndpoint(server.URL+"/badjson", true)
			Expect(err).To(HaveOccurred())
			Expect(endpoint).To(BeZero())
		})
	})
}
