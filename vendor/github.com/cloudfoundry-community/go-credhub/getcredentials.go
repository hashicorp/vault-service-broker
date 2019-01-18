package credhub

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// GetByID will look up a credental by its ID. Since each version of a named
// credential has a different ID, this will always return at most one value.
func (c *Client) GetByID(id string) (*Credential, error) {
	resp, err := c.hc.Get(c.url + "/api/v1/data/" + id)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, errors.New("credential not found")
	}

	marshaller := json.NewDecoder(resp.Body)

	cred := new(Credential)
	if err = marshaller.Decode(cred); err != nil {
		return nil, err
	}

	return cred, nil
}

// GetAllByName will return all versions of a credential, sorted in descending
// order by their created date.
func (c *Client) GetAllByName(name string) ([]Credential, error) {
	return c.getByName(name, false, -1)
}

// GetVersionsByName will return the latest numVersions versions of a given
// credential, still sorted in descending order by their created date.
func (c *Client) GetVersionsByName(name string, numVersions int) ([]Credential, error) {
	return c.getByName(name, false, numVersions)
}

// GetLatestByName will return the current version of a credential. It will return
// at most one item.
func (c *Client) GetLatestByName(name string) (*Credential, error) {
	creds, err := c.getByName(name, true, -1)
	if err != nil {
		return nil, err
	}

	return &creds[0], nil
}

func (c *Client) getByName(name string, latest bool, numVersions int) ([]Credential, error) {
	var retBody struct {
		Data []Credential `json:"data"`
	}

	chURL := c.url + "/api/v1/data?"

	params := url.Values{}
	params.Add("name", name)

	if latest {
		params.Add("current", "true")
	}

	if numVersions > 0 {
		params.Add("versions", fmt.Sprint(numVersions))
	}

	chURL += params.Encode()
	resp, err := c.hc.Get(chURL)
	if err != nil {
		return retBody.Data, err
	}

	if resp.StatusCode == 404 {
		return nil, errors.New("Name Not Found")
	}

	marshaller := json.NewDecoder(resp.Body)

	err = marshaller.Decode(&retBody)
	if err != nil {
		return nil, err
	}

	data := retBody.Data
	sort.Slice(data, func(i, j int) bool {
		less := strings.Compare(data[i].Created, data[j].Created)
		// we want to sort in reverse order, so return the opposite of what you'd normally do
		return less > 0
	})

	return retBody.Data, err
}
