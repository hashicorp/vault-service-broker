package credhub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// AddPermissions adds permissions to a credential. Note that this method is *not* idempotent.
func (c *Client) AddPermissions(credentialName string, newPerms []Permission) ([]Permission, error) {
	type permbody struct {
		Name        string       `json:"credential_name"`
		Permissions []Permission `json:"permissions"`
	}

	request := permbody{
		Name:        credentialName,
		Permissions: newPerms,
	}

	// request fully conforms to the go json spec, so an error can't occur
	body, _ := json.Marshal(request)

	req, err := http.NewRequest("POST", c.url+"/api/v1/permissions", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response permbody

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&response)
	if err != nil {
		return nil, err
	}

	return response.Permissions, nil
}

// DeletePermissions deletes permissions from a credential. Note that this method
// is *not* idempotent
func (c *Client) DeletePermissions(credentialName, actorID string) error {
	chURL := c.url + "/api/v1/permissions"

	req, err := http.NewRequest("DELETE", chURL, nil)
	if err != nil {
		return err
	}

	params := make(url.Values)
	params.Add("credential_name", credentialName)
	params.Add("actor", actorID)
	req.URL.RawQuery = params.Encode()

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		return fmt.Errorf("expected return code 204, got %d", resp.StatusCode)
	}

	return nil
}
