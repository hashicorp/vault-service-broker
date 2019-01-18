package credhub

// Client interacts with the Credhub API. It provides methods for all available
// endpoints
type Client struct {
	url string
	hc  HTTPClient
}

// New creates a new Credhub client. You must bring an *http.Client that will
// negotiate authentication and authorization for you. See the examples for more
// information.
func New(credhubURL string, hc HTTPClient) *Client {
	return &Client{
		url: credhubURL,
		hc:  hc,
	}
}
