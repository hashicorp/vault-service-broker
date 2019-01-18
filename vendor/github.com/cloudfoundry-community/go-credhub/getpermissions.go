package credhub

import (
	"encoding/json"
	"errors"
	"net/url"
)

// GetPermissions returns the permissions of a credential. Permissions consist of
// an actor (See https://github.com/cloudfoundry-incubator/credhub/blob/master/docs/authentication-identities.md
// for more information on actor identities) and Operations
func (c *Client) GetPermissions(credentialName string) ([]Permission, error) {
	params := make(url.Values)
	params.Add("credential_name", credentialName)

	resp, err := c.hc.Get(c.url + "/api/v1/permissions?" + params.Encode())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, errors.New("credential not found")
	}

	retBody := struct {
		CN          string       `json:"credential_name"`
		Permissions []Permission `json:"permissions"`
	}{}

	marshaller := json.NewDecoder(resp.Body)

	err = marshaller.Decode(&retBody)
	return retBody.Permissions, err
}
