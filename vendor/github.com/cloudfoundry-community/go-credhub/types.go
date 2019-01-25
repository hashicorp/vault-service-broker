package credhub

import (
	"encoding/json"
	"errors"
)

// CredentialType is the list of valid types of credentials Credhub supports
type CredentialType string

const (
	// Value - A generic value
	Value CredentialType = "value"
	// Password - A password that can be (re-)generated
	Password CredentialType = "password"
	// User - A username, password, and password hash
	User CredentialType = "user"
	// JSON - An arbitrary block of JSON
	JSON CredentialType = "json"
	// RSA - A public/private key pair
	RSA CredentialType = "rsa"
	// SSH - An SSH private key, public key (in OpenSSH format), and public key fingerprint
	SSH CredentialType = "ssh"
	// Certificate - A private key, associated certificate, and CA
	Certificate CredentialType = "certificate"
)

// OverwriteMode is the list of valid "mode" arguments
type OverwriteMode string

const (
	// Overwrite will overwrite an existing credential on Set or Generate
	Overwrite OverwriteMode = "overwrite"
	// NoOverwrite will not overwrite an existing credential on Set or Generate
	NoOverwrite OverwriteMode = "no-overwrite"
	// Converge will only overwrite an existing credential if the parameters have changed
	Converge OverwriteMode = "converge"
)

// Operation is the list of valid operations
type Operation string

const (
	// Read operation allows the actor to fetch and view credentials
	Read Operation = "read"

	// Write operation allows the actor to create, update, and generate credentials
	Write Operation = "write"

	// Delete operation allows the actor to delete credentials
	Delete Operation = "delete"

	// ReadACL operation allows the actor to view all permissions on a given credential
	ReadACL Operation = "read_acl"

	// WriteACL operation allows the actor to create and delete permissions on a given credential
	WriteACL Operation = "write_acl"
)

// Credential is the base type that the credential-based methods of Client will
// return.
type Credential struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Created      string         `json:"version_created_at"`
	Type         CredentialType `json:"type,omitempty"`
	Value        interface{}    `json:"value,omitempty"`
	remarshalled bool
}

// Permission represents the operations an actor is allowed to perform on a
// credential. See https://github.com/cloudfoundry-incubator/credhub/blob/master/docs/authentication-identities.md for
// more information on actor identities
type Permission struct {
	Actor      string      `json:"actor"`
	Operations []Operation `json:"operations"`
}

// UserValueType is what a user type credential will have. Use UserValue() to
// get this from a user type Credential
type UserValueType struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	PasswordHash string `json:"password_hash"`
}

// RSAValueType is what a rsa type credential will have. Use RSAValue() to
// get this from a rsa type Credential
type RSAValueType struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

// SSHValueType is what a ssh type credential will have. Use SSHValue() to
// get this from a ssh type Credential
type SSHValueType struct {
	PublicKey            string `json:"public_key"`
	PrivateKey           string `json:"private_key"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
}

// CertificateValueType is what a certificate type credential will have. Use
// CertificateValue() to get this from a certificate type credential.
type CertificateValueType struct {
	CA          string `json:"ca"`
	PrivateKey  string `json:"private_key"`
	Certificate string `json:"certificate"`
}

// UserValue will remarshal a credential so that its Value is a UserValueType.
// Use this method to get the UserValueType from the credential. Subsequent calls
// to this return the remarshalled struct.
func UserValue(cred Credential) (UserValueType, error) {
	def := UserValueType{}
	switch cred.Type {
	case User:
		if !cred.remarshalled {
			val := UserValueType{}
			buf, err := json.Marshal(cred.Value)
			if err != nil {
				return def, err
			}

			err = json.Unmarshal(buf, &val)
			if err != nil {
				return def, err
			}

			cred.Value = val
			cred.remarshalled = true
		}

		return cred.Value.(UserValueType), nil
	default:
		return def, errors.New(`only "user" type credentials have UserValueType values`)
	}
}

// RSAValue will remarshal a credential so that its Value is a RSAValueType.
// Use this method to get the RSAValueType from the credential. Subsequent calls
// to this return the remarshalled struct.
func RSAValue(cred Credential) (RSAValueType, error) {
	def := RSAValueType{}
	switch cred.Type {
	case RSA:
		if !cred.remarshalled {
			val := RSAValueType{}
			buf, err := json.Marshal(cred.Value)
			if err != nil {
				return def, err
			}

			err = json.Unmarshal(buf, &val)
			if err != nil {
				return def, err
			}

			cred.Value = val
			cred.remarshalled = true

		}

		return cred.Value.(RSAValueType), nil
	default:
		return def, errors.New(`only "rsa" type credentials have RSAValueType values`)
	}
}

// SSHValue will remarshal a credential so that its Value is a SSHValueType.
// Use this method to get the SSHValueType from the credential. Subsequent calls
// to this return the remarshalled struct.
func SSHValue(cred Credential) (SSHValueType, error) {
	def := SSHValueType{}
	switch cred.Type {
	case SSH:
		if !cred.remarshalled {
			val := SSHValueType{}
			buf, err := json.Marshal(cred.Value)
			if err != nil {
				return def, err
			}

			err = json.Unmarshal(buf, &val)
			if err != nil {
				return def, err
			}

			cred.Value = val
			cred.remarshalled = true
		}

		return cred.Value.(SSHValueType), nil
	default:
		return def, errors.New(`only "ssh" type credentials have SSHValueType values`)
	}
}

// CertificateValue will remarshal a credential so that its Value is a CertificateValueType.
// Use this method to get the CertificateValueType from the credential. Subsequent calls
// to this return the remarshalled struct.
func CertificateValue(cred Credential) (CertificateValueType, error) {
	def := CertificateValueType{}
	switch cred.Type {
	case Certificate:
		if !cred.remarshalled {
			val := CertificateValueType{}
			buf, err := json.Marshal(cred.Value)
			if err != nil {
				return def, err
			}

			err = json.Unmarshal(buf, &val)
			if err != nil {
				return def, err
			}

			cred.Value = val
			cred.remarshalled = true
		}

		return cred.Value.(CertificateValueType), nil
	default:
		return def, errors.New(`only "certificate" type credentials have CertificateValueType values`)
	}
}
