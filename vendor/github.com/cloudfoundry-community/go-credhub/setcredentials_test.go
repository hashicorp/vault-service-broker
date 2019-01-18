package credhub_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	credhub "github.com/cloudfoundry-community/go-credhub"
)

func TestSetCredentials(t *testing.T) {
	spec.Run(t, "SetCredentials", testSetCredentials, spec.Report(report.Terminal{}))
}

func testSetCredentials(t *testing.T, when spec.G, it spec.S) {
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

	when("Testing Set credhub.Credential", func() {
		it("should receive the same item it sent, but with a timestamp", func() {
			cred := credhub.Credential{
				Name: "/sample-set",
				Type: "user",
				Value: credhub.UserValueType{
					Username:     "me",
					Password:     "super-secret",
					PasswordHash: "somestring",
				},
			}

			newCred, err := chClient.Set(cred, credhub.Overwrite, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(newCred.Created).NotTo(BeEmpty())
			Expect(newCred.ID).NotTo(BeEmpty())
		})
		it("should receive an old credential", func() {
			cred := credhub.Credential{
				Name: "/sample-set",
				Type: "user",
				Value: credhub.UserValueType{
					Username:     "me",
					Password:     "super-secret",
					PasswordHash: "somestring",
				},
			}

			newCred, err := chClient.Set(cred, credhub.NoOverwrite, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(newCred.Created).To(Not(BeEmpty()))
			v, err := credhub.UserValue(*newCred)
			Expect(err).To(Not(HaveOccurred()))
			Expect(v.Password).To(BeEquivalentTo("old"))
			Expect(newCred.ID).To(BeEquivalentTo("6ba7b810-9dad-11d1-80b4-00c04fd430c8"))
		})
		it("should receive an old credential if converging without changes", func() {
			cred := credhub.Credential{
				Name: "/sample-set",
				Type: "user",
				Value: credhub.UserValueType{
					Username:     "me",
					Password:     "super-secret",
					PasswordHash: "somestring",
				},
			}

			newCred, err := chClient.Set(cred, credhub.Converge, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(newCred.Created).To(Not(BeEmpty()))
			Expect(newCred.ID).To(BeEquivalentTo("6ba7b810-9dad-11d1-80b4-00c04fd430c8"))
		})
		it("should receive a new credential if converging with changes", func() {
			cred := credhub.Credential{
				Name: "/sample-set",
				Type: "user",
				Value: credhub.UserValueType{
					Username:     "me",
					Password:     "new-super-secret",
					PasswordHash: "somestring",
				},
			}

			newCred, err := chClient.Set(cred, credhub.Converge, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(newCred.Created).To(Not(BeEmpty()))
			Expect(newCred.ID).To(Not(BeEquivalentTo("6ba7b810-9dad-11d1-80b4-00c04fd430c8")))
		})
	})

	when("testing edge cases", func() {
		when("an error occurs creating the HTTP request", func() {
			it("fails", func() {
				chClient = credhub.New("badscheme://bad_hsot\\", http.DefaultClient)
				cred, err := chClient.Set(credhub.Credential{}, credhub.Overwrite, nil)
				Expect(err).To(HaveOccurred())
				Expect(cred).To(BeNil())
			})
		})

		when("an error occurs on the http round trip", func() {
			it("fails", func() {
				chClient = credhub.New(server.URL, &http.Client{Transport: &errorRoundTripper{}})
				cred, err := chClient.Set(credhub.Credential{}, credhub.Overwrite, nil)
				Expect(err).To(HaveOccurred())
				Expect(cred).To(BeNil())
			})
		})
	})
}
