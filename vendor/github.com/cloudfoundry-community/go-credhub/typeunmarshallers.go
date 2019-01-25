package credhub

import (
	"encoding/json"
	"errors"
)

// UnmarshalJSON will unmarshal the JSON and strictly conform to the struct
func (u *UserValueType) UnmarshalJSON(b []byte) error {
	m, err := baseUnmarshal(b)

	if err != nil {
		return err
	}

	for key := range m {
		switch key {
		case "username":
			username, ok := m[key].(string)
			if !ok {
				u.Username = ""
				u.Password = ""
				u.PasswordHash = ""
				return errors.New("username is not a string")
			}
			u.Username = username
			delete(m, key)
		case "password":
			password, ok := m[key].(string)
			if !ok {
				u.Username = ""
				u.Password = ""
				u.PasswordHash = ""
				return errors.New("password is not a string")
			}
			u.Password = password
			delete(m, key)
		case "password_hash":
			hash, ok := m[key].(string)
			if !ok {
				u.Username = ""
				u.Password = ""
				u.PasswordHash = ""
				return errors.New("password_hash is not a string")
			}
			u.PasswordHash = hash
			delete(m, key)
		}
	}

	if len(m) > 0 {
		return errors.New("extra fields leftover")
	}

	return nil
}

// UnmarshalJSON will unmarshal the JSON and strictly conform to the struct
func (r *RSAValueType) UnmarshalJSON(b []byte) error {
	m, err := baseUnmarshal(b)

	if err != nil {
		return err
	}

	for key := range m {
		switch key {
		case "public_key":
			public, ok := m[key].(string)
			if !ok {
				r.PublicKey = ""
				r.PrivateKey = ""
				return errors.New("public key is not a string")
			}
			r.PublicKey = public
			delete(m, key)
		case "private_key":
			private, ok := m[key].(string)
			if !ok {
				r.PublicKey = ""
				r.PrivateKey = ""
				return errors.New("private key is not a string")
			}
			r.PrivateKey = private
			delete(m, key)
		}
	}

	if len(m) > 0 {
		return errors.New("extra fields leftover")
	}

	return nil
}

// UnmarshalJSON will unmarshal the JSON and strictly conform to the struct
func (s *SSHValueType) UnmarshalJSON(b []byte) error {
	m, err := baseUnmarshal(b)

	if err != nil {
		return err
	}

	for key := range m {
		switch key {
		case "public_key":
			public, ok := m[key].(string)
			if !ok {
				s.PublicKey = ""
				s.PrivateKey = ""
				s.PublicKeyFingerprint = ""
				return errors.New("public key is not a string")
			}
			s.PublicKey = public
			delete(m, key)
		case "private_key":
			private, ok := m[key].(string)
			if !ok {
				s.PublicKey = ""
				s.PrivateKey = ""
				s.PublicKeyFingerprint = ""
				return errors.New("private key is not a string")
			}
			s.PrivateKey = private
			delete(m, key)
		case "public_key_fingerprint":
			fingerprint, ok := m[key].(string)
			if !ok {
				s.PublicKey = ""
				s.PrivateKey = ""
				s.PublicKeyFingerprint = ""
				return errors.New("public key fingerprint is not a string")
			}
			s.PublicKeyFingerprint = fingerprint
			delete(m, key)
		}
	}

	if len(m) > 0 {
		return errors.New("extra fields leftover")
	}

	return nil
}

// UnmarshalJSON will unmarshal the JSON and strictly conform to the struct
func (c *CertificateValueType) UnmarshalJSON(b []byte) error {
	m, err := baseUnmarshal(b)

	if err != nil {
		return err
	}

	for key := range m {
		switch key {
		case "ca":
			ca, ok := m[key].(string)
			if !ok {
				c.CA = ""
				c.PrivateKey = ""
				c.Certificate = ""
				return errors.New("CA is not a string")
			}
			c.CA = ca
			delete(m, key)
		case "private_key":
			private, ok := m[key].(string)
			if !ok {
				c.CA = ""
				c.PrivateKey = ""
				c.Certificate = ""
				return errors.New("private key is not a string")
			}
			c.PrivateKey = private
			delete(m, key)
		case "certificate":
			certificate, ok := m[key].(string)
			if !ok {
				c.CA = ""
				c.PrivateKey = ""
				c.Certificate = ""
				return errors.New("certificate is not a string")
			}
			c.Certificate = certificate
			delete(m, key)
		}
	}

	if len(m) > 0 {
		return errors.New("extra fields leftover")
	}

	return nil
}

func baseUnmarshal(b []byte) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	e := json.Unmarshal(b, &m)

	return m, e
}
