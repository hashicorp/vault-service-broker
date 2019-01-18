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

func TestGetCredentials(t *testing.T) {
	spec.Run(t, "GetCredentials", testGetCredentials, spec.Report(report.Terminal{}))
}

func testGetCredentials(t *testing.T, when spec.G, it spec.S) {
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

	valueByNameTests := func(latest bool, num int) func() {
		if latest {
			return func() {
				cred, err := chClient.GetLatestByName("/concourse/common/sample-value")
				Expect(err).To(Not(HaveOccurred()))
				Expect(cred.Value).To(BeEquivalentTo("sample2"))
			}
		} else if num <= 0 {
			return func() {
				creds, err := chClient.GetAllByName("/concourse/common/sample-value")
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(creds)).To(Equal(3))
			}
		} else {
			return func() {
				creds, err := chClient.GetVersionsByName("/concourse/common/sample-value", num)
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(creds)).To(Equal(num))
			}
		}
	}

	passwordByNameTests := func(latest bool, num int) func() {
		if latest {
			return func() {
				cred, err := chClient.GetLatestByName("/concourse/common/sample-password")
				Expect(err).To(Not(HaveOccurred()))
				Expect(cred.Value).To(BeEquivalentTo("sample1"))
			}
		} else if num <= 0 {
			return func() {
				creds, err := chClient.GetAllByName("/concourse/common/sample-password")
				Expect(err).To(BeNil())
				Expect(len(creds)).To(Equal(3))
				Expect(creds[2].Value).To(BeEquivalentTo("sample2"))
			}
		} else {
			return func() {
				creds, err := chClient.GetVersionsByName("/concourse/common/sample-value", num)
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(creds)).To(Equal(num))
			}
		}
	}

	jsonByNameTests := func(latest bool, num int) func() {
		if latest {
			return func() {
				cred, err := chClient.GetLatestByName("/concourse/common/sample-json")
				Expect(err).To(Not(HaveOccurred()))

				rawVal := cred.Value
				val, ok := rawVal.(map[string]interface{})
				Expect(ok).To(BeTrue())

				Expect(val["foo"]).To(BeEquivalentTo("bar"))
			}
		} else if num <= 0 {
			return func() {
				creds, err := chClient.GetAllByName("/concourse/common/sample-json")
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(creds)).To(Equal(3))

				intf := creds[2].Value
				val, ok := intf.([]interface{})
				Expect(ok).To(BeTrue())

				Expect(int(val[0].(float64))).To(Equal(1))
				Expect(int(val[1].(float64))).To(Equal(2))
			}
		} else {
			return func() {
				creds, err := chClient.GetVersionsByName("/concourse/common/sample-value", num)
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(creds)).To(Equal(num))
			}
		}
	}

	getUserByName := func() {
		creds, err := chClient.GetAllByName("/concourse/common/sample-user")
		Expect(err).To(Not(HaveOccurred()))
		Expect(len(creds)).To(Equal(1))

		cred := creds[0]

		var val credhub.UserValueType
		val, err = credhub.UserValue(cred)
		Expect(err).To(Not(HaveOccurred()))
		Expect(val.Username).To(Equal("me"))
	}

	getSSHByName := func() {
		creds, err := chClient.GetAllByName("/concourse/common/sample-ssh")
		Expect(err).To(Not(HaveOccurred()))
		Expect(len(creds)).To(Equal(1))

		cred := creds[0]
		var val credhub.SSHValueType
		val, err = credhub.SSHValue(cred)
		Expect(err).To(Not(HaveOccurred()))
		Expect(val.PublicKey).To(HavePrefix("ssh-rsa"))
	}

	getRSAByName := func() {
		creds, err := chClient.GetAllByName("/concourse/common/sample-rsa")
		Expect(err).To(Not(HaveOccurred()))
		Expect(len(creds)).To(Equal(1))

		cred := creds[0]
		var val credhub.RSAValueType
		val, err = credhub.RSAValue(cred)
		Expect(err).To(Not(HaveOccurred()))
		Expect(val.PrivateKey).To(HavePrefix("-----BEGIN PRIVATE KEY-----"))

	}

	getNonexistentName := func() {
		_, err := chClient.GetAllByName("/concourse/common/not-real")
		Expect(err).To(HaveOccurred())
	}

	getCertificateByName := func() {
		creds, err := chClient.GetAllByName("/concourse/common/sample-certificate")
		Expect(err).To(Not(HaveOccurred()))
		Expect(len(creds)).To(Equal(1))

		cred := creds[0]
		var val credhub.CertificateValueType
		val, err = credhub.CertificateValue(cred)
		Expect(err).To(Not(HaveOccurred()))
		Expect(val.Certificate).To(HavePrefix("-----BEGIN CERTIFICATE-----"))
	}

	when("Testing Get By Name", func() {
		it("should get a 'value' type credential", valueByNameTests(false, -1))
		it("should get a 'password' type credential", passwordByNameTests(false, -1))
		it("should get a 'json' type credential", jsonByNameTests(false, -1))
		it("should get a 'user' type credential", getUserByName)
		it("should get a 'ssh' type credential", getSSHByName)
		it("should get a 'rsa' type credential", getRSAByName)
		it("should get a 'certificate' type credential", getCertificateByName)
		it("should not get a credential that doesn't exist", getNonexistentName)
	})

	when("Testing Get Latest By Name", func() {
		it("should get a 'value' type credential", valueByNameTests(true, -1))
		it("should get a 'password' type credential", passwordByNameTests(true, -1))
		it("should get a 'json' type credential", jsonByNameTests(true, -1))
	})

	when("Testing Get Latest By Name", func() {
		it("should get a 'value' type credential", valueByNameTests(false, 2))
		it("should get a 'password' type credential", passwordByNameTests(false, 2))
		it("should get a 'json' type credential", jsonByNameTests(false, 2))
	})

	when("Testing Get By ID", func() {
		it("should get an item with a valid ID", func() {
			cred, err := chClient.GetByID("1234")
			Expect(err).To(Not(HaveOccurred()))
			Expect(cred.Name).To(BeEquivalentTo("/by-id"))

			badcred, err := chClient.GetByID("4567")
			Expect(err).To(HaveOccurred())
			Expect(badcred).To(BeNil())
		})
	})

	when("testing edge cases", func() {
		when("an error happens during the http round trip", func() {
			it.Before(func() {
				chClient = credhub.New(server.URL, &http.Client{Transport: &errorRoundTripper{}})
			})

			when("getting a credential by id", func() {
				it("fails", func() {
					cred, err := chClient.GetByID("1234")
					Expect(err).To(HaveOccurred())
					Expect(cred).To(BeNil())
				})
			})

			when("getting all versions of a credential by name", func() {
				it("fails", func() {
					cred, err := chClient.GetAllByName("test")
					Expect(err).To(HaveOccurred())
					Expect(cred).To(BeNil())
				})
			})

			when("getting the latest version of a credential by name", func() {
				it("fails", func() {
					cred, err := chClient.GetLatestByName("test")
					Expect(err).To(HaveOccurred())
					Expect(cred).To(BeNil())
				})
			})
		})

		when("bad json is returned", func() {
			it.Before(func() {
				chClient = credhub.New(server.URL+"/badjson", getAuthenticatedClient(server.Client()))
			})

			when("getting a credential by id", func() {
				it("fails", func() {
					cred, err := chClient.GetByID("1234")
					Expect(err).To(HaveOccurred())
					Expect(cred).To(BeNil())
				})
			})

			when("getting all versions of a credential by name", func() {
				it("fails", func() {
					cred, err := chClient.GetAllByName("test")
					Expect(err).To(HaveOccurred())
					Expect(cred).To(BeNil())
				})
			})
		})
	})
}
