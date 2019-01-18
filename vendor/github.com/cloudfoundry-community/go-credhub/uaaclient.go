package credhub

import (
	"net/http"

	uaa "code.cloudfoundry.org/uaa-go-client"
)

/*

NewUAAAuthClient creates a UAAAuthClient.

Example usage:

	cfg := &config.Config{
		ClientName:       "client-name",
		ClientSecret:     "client-secret",
		UaaEndpoint:      "https://uaa.service.cf.internal:8443",
		SkipVerification: true,
	}

	uaaClient, err = client.NewClient(logger, cfg, clock)
	if err != nil {
		...
	}

	client := NewUAAAuthClient(http.DefaultClient(), uaaClient)

See github.com/cloudfoundry-community/uaa-go-client for more examples of instantiating the UAA client.

*/
func NewUAAAuthClient(hc HTTPClient, ua uaa.Client) HTTPClient {
	return &UAAAuthClient{
		hc: hc,
		uc: ua,
	}
}

// UAAAuthClient is a thin wrapper around an http.Client
// that handles authenticating and renewing tokens
// provided via UAA.
type UAAAuthClient struct {
	hc HTTPClient
	uc uaa.Client
}

func (c *UAAAuthClient) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *UAAAuthClient) Do(req *http.Request) (*http.Response, error) {
	return c.do(req)
}

func (c *UAAAuthClient) do(req *http.Request) (*http.Response, error) {

	// FetchToken has internal logic where if the token isn't expired,
	// it'll pull a cached one; otherwise it'll make a remote call to
	// get a valid current one.
	token, err := c.uc.FetchToken(false)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "bearer "+token.AccessToken)
	return c.hc.Do(req)
}
