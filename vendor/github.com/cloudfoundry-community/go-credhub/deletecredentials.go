package credhub

import (
	"fmt"
	"net/http"
)

// Delete deletes a credential by name
func (c *Client) Delete(name string) error {
	chURL := c.url + "/api/v1/data?name=" + name
	req, err := http.NewRequest("DELETE", chURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("expected return code 204, got %d", resp.StatusCode)
	}

	return nil
}
