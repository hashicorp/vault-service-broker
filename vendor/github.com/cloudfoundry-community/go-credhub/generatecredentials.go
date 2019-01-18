package credhub

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Generate will create a credential in Credhub. Currently does not work for the
// Value or JSON credential types. See https://credhub-api.cfapps.io/#generate-credentials
// for more information about available parameters.
func (c *Client) Generate(name string, credentialType CredentialType, parameters map[string]interface{}) (*Credential, error) {
	reqBody := make(map[string]interface{})
	reqBody["name"] = name
	reqBody["type"] = credentialType
	reqBody["parameters"] = parameters

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	var req *http.Request
	req, err = http.NewRequest("POST", c.url+"/api/v1/data", bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}

	cred := new(Credential)
	unmarshaller := json.NewDecoder(resp.Body)
	err = unmarshaller.Decode(cred)

	return cred, err
}

// Regenerate will generate new values for credentials using the same parameters
// as the stored value. All RSA and SSH credentials may be regenerated. Password
// and user credentials must have been generated to enable regeneration.
// Statically set certificates may be regenerated if they are self-signed or if
// the CA name has been set to a stored CA certificate.
func (c *Client) Regenerate(name string) (*Credential, error) {
	reqBody := struct {
		Name string `json:"name"`
	}{
		Name: name,
	}

	// there's no way that this will ever return an error, so ignore the error
	buf, _ := json.Marshal(reqBody)

	var req *http.Request
	req, err := http.NewRequest("POST", c.url+"/api/v1/data/regenerate", bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}

	cred := new(Credential)
	unmarshaller := json.NewDecoder(resp.Body)
	err = unmarshaller.Decode(cred)

	return cred, err
}
