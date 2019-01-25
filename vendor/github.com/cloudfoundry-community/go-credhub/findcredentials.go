package credhub

import (
	"encoding/json"
	"io/ioutil"
)

// ListAllPaths lists all paths that have credentials that have that prefix.
// Use in conjunction with FindByPath() to list all credentials
func (c *Client) ListAllPaths() ([]string, error) {
	var retBody struct {
		Paths []struct {
			Path string `json:"path"`
		} `json:"paths"`
	}

	resp, err := c.hc.Get(c.url + "/api/v1/data?paths=true")
	if err != nil {
		return nil, err
	}

	marshaller := json.NewDecoder(resp.Body)

	if err = marshaller.Decode(&retBody); err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(retBody.Paths))
	for _, path := range retBody.Paths {
		paths = append(paths, path.Path)
	}

	return paths, nil
}

// FindByPath retrieves a list of stored credential names which are within the
// specified path. This method does not traverse sub-paths.
func (c *Client) FindByPath(path string) ([]Credential, error) {
	var retBody struct {
		Credentials []Credential `json:"credentials"`
	}

	resp, err := c.hc.Get(c.url + "/api/v1/data?path=" + path)
	if err != nil {
		return nil, err
	}

	buf, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(buf, &retBody)
	return retBody.Credentials, err
}

// FindByPartialName retrieves a list of stored credential names which contain the search.
func (c *Client) FindByPartialName(partialName string) ([]Credential, error) {
	var retBody struct {
		Credentials []Credential `json:"credentials"`
	}

	resp, err := c.hc.Get(c.url + "/api/v1/data?name-like=" + partialName)
	if err != nil {
		return nil, err
	}

	marshaller := json.NewDecoder(resp.Body)

	err = marshaller.Decode(&retBody)
	return retBody.Credentials, err
}
