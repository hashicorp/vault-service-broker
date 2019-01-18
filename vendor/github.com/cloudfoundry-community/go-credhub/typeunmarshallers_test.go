package credhub_test

import (
	"encoding/json"
	"testing"

	credhub "github.com/cloudfoundry-community/go-credhub"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestStrictTypeUnmarshalling(t *testing.T) {
	spec.Run(t, "StrictTypeUnmarshalling", testStrictTypeUnmarshalling, spec.Report(report.Terminal{}))
}

func testStrictTypeUnmarshalling(t *testing.T, when spec.G, it spec.S) {
	it.Before(func() {
		RegisterTestingT(t)
	})

	when("unmarshalling UserValueTypes", func() {
		it("works when exactly the right fields are in the JSON", func() {
			j := `{"username":"josh", "password": "secret", "password_hash": "secrethash"}`

			val := credhub.UserValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).NotTo(HaveOccurred())
			Expect(val.Username).To(Equal("josh"))
			Expect(val.Password).To(Equal("secret"))
			Expect(val.PasswordHash).To(Equal("secrethash"))
		})

		it("fails when the username is not a string", func() {
			j := `{"username": true, "password": "secret", "password_hash": "secrethash"}`
			val := credhub.UserValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the password is not a string", func() {
			j := `{"username": "josh", "password": 3, "password_hash": "secrethash"}`
			val := credhub.UserValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the password hash is not a string", func() {
			j := `{"username": "josh", "password": "secret", "password_hash": []}`
			val := credhub.UserValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the json is invalid", func() {
			j := `{invalid}`
			val := &credhub.UserValueType{}
			err := val.UnmarshalJSON([]byte(j))
			Expect(err).To(HaveOccurred())
			Expect(*val).To(BeZero())
		})
	})

	when("unmarshalling RSAValueType", func() {
		it("works when exactly the right fields are in the JSON", func() {
			j := `{"public_key":"somepublickey", "private_key": "someprivatekey"}`

			val := credhub.RSAValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).NotTo(HaveOccurred())
			Expect(val.PublicKey).To(Equal("somepublickey"))
			Expect(val.PrivateKey).To(Equal("someprivatekey"))
		})

		it("fails when the public key is not a string", func() {
			j := `{"public_key":false, "private_key": "someprivatekey"}`
			val := credhub.RSAValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the private key is not a string", func() {
			j := `{"public_key":"somepublickey", "private_key": 3}`
			val := credhub.RSAValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the json is invalid", func() {
			j := `{invalid}`
			val := &credhub.RSAValueType{}
			err := val.UnmarshalJSON([]byte(j))
			Expect(err).To(HaveOccurred())
			Expect(*val).To(BeZero())
		})
	})

	when("unmarshalling SSHValueType", func() {
		it("works when exactly the right fields are in the JSON", func() {
			j := `{"public_key":"somepublickey", "private_key": "someprivatekey", "public_key_fingerprint": "fp"}`

			val := credhub.SSHValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).NotTo(HaveOccurred())
			Expect(val.PublicKey).To(Equal("somepublickey"))
			Expect(val.PrivateKey).To(Equal("someprivatekey"))
			Expect(val.PublicKeyFingerprint).To(Equal("fp"))
		})

		it("fails when the public key is not a string", func() {
			j := `{"public_key":false, "private_key": "someprivatekey", "public_key_fingerprint": "fp"}`
			val := credhub.SSHValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the private key is not a string", func() {
			j := `{"public_key":"somepublickey", "private_key": 3, "public_key_fingerprint": "fp"}`
			val := credhub.SSHValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the fingerprint is not a string", func() {
			j := `{"public_key":"public_key", "private_key": "someprivatekey", "public_key_fingerprint": []}`
			val := credhub.SSHValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the json is invalid", func() {
			j := `{invalid}`
			val := &credhub.SSHValueType{}
			err := val.UnmarshalJSON([]byte(j))
			Expect(err).To(HaveOccurred())
			Expect(*val).To(BeZero())
		})
	})

	when("unmarshalling CertificateValueType", func() {
		it("works when exactly the right fields are in the JSON", func() {
			j := `{"ca":"someca", "private_key": "someprivatekey", "certificate": "somecert"}`

			val := credhub.CertificateValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).NotTo(HaveOccurred())
			Expect(val.CA).To(Equal("someca"))
			Expect(val.PrivateKey).To(Equal("someprivatekey"))
			Expect(val.Certificate).To(Equal("somecert"))
		})

		it("fails when the ca is not a string", func() {
			j := `{"ca":3, "private_key": "someprivatekey", "certificate": "somecert"}`
			val := credhub.CertificateValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the private key is not a string", func() {
			j := `{"ca":"someca", "private_key": false, "certificate": "somecert"}`
			val := credhub.CertificateValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the certificate is not a string", func() {
			j := `{"ca":"someca", "private_key": "someprivatekey", "certificate": {}}`
			val := credhub.CertificateValueType{}
			err := json.Unmarshal([]byte(j), &val)
			Expect(err).To(HaveOccurred())
			Expect(val).To(BeZero())
		})

		it("fails when the json is invalid", func() {
			j := `{invalid}`
			val := &credhub.CertificateValueType{}
			err := val.UnmarshalJSON([]byte(j))
			Expect(err).To(HaveOccurred())
			Expect(*val).To(BeZero())
		})
	})
}
