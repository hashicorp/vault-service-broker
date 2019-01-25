package credhub_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	credhub "github.com/cloudfoundry-community/go-credhub"
)

func TestInterpolateCredentials(t *testing.T) {
	spec.Run(t, "InterpolateCredentials", testInterpolateCredentials, spec.Report(report.Terminal{}))
}

func testInterpolateCredentials(t *testing.T, when spec.G, it spec.S) {
	var (
		server   *httptest.Server
		chClient *credhub.Client
	)

	it.Before(func() {
		RegisterTestingT(t)
		server = mockCredhubServer()
		chClient = credhub.New(server.URL, getAuthenticatedClient(server.Client()))
	})

	it.After(func() {
		server.Close()
	})

	when("interpolating VCAP services credentials", func() {
		it("works", func() {
			vcapServices := `
      {
        "p-config-server": [
        {
          "credentials": {
            "credhub-ref": "/service-cred-ref"
          },
          "label": "p-config-server",
          "name": "config-server",
          "plan": "standard",
          "provider": null,
          "syslog_drain_url": null,
          "tags": [
          "configuration",
          "spring-cloud"
          ],
          "volume_mounts": []
        }
        ]
      }
      `

			cred, err := chClient.GetLatestByName("/service-cred-ref")
			Expect(err).NotTo(HaveOccurred())

			interpolated, err := chClient.InterpolateCredentials(vcapServices)
			Expect(err).NotTo(HaveOccurred())
			Expect(vcapServicesDeepEnoughEquals(vcapServices, interpolated)).To(BeTrue())

			interpolatedObj := make(map[string][]map[string]interface{})
			err = json.Unmarshal([]byte(interpolated), &interpolatedObj)
			Expect(err).NotTo(HaveOccurred())

			resolvedCred := interpolatedObj["p-config-server"][0]["credentials"]
			Expect(resolvedCred).To(BeEquivalentTo(cred.Value))
		})
	})

	when("testing edge cases", func() {
		when("getting invalid VCAP_SERVICES json", func() {
			it("fails", func() {
				vcapServices := `{invalid}`
				interpolated, err := chClient.InterpolateCredentials(vcapServices)
				Expect(err).To(HaveOccurred())
				Expect(interpolated).To(BeZero())
			})
		})

		when("the credential ref does not exist", func() {
			it("fails", func() {
				vcapServices := `
	      {
	        "p-config-server": [
	        {
	          "credentials": {
	            "credhub-ref": "/this-does-not-exist"
	          }
	        }
	        ]
	      }
	      `

				interpolated, err := chClient.InterpolateCredentials(vcapServices)
				Expect(err).To(HaveOccurred())
				Expect(interpolated).To(BeZero())
			})
		})
	})
}
