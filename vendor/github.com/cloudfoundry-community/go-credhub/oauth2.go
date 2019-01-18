package credhub

import (
	"crypto/tls"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
)

// UAAEndpoint will get the info about the UAA server associated with the specified Credhub
func UAAEndpoint(credhubURL string, skipTLSVerify bool) (oauth2.Endpoint, error) {
	endpoint := oauth2.Endpoint{}

	baseClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify},
		},
	}

	r, err := baseClient.Get(credhubURL + "/info")
	if err != nil {
		return endpoint, err
	}

	var body struct {
		AuthServer struct {
			URL string `json:"url"`
		} `json:"auth-server"`
	}
	decoder := json.NewDecoder(r.Body)

	if err = decoder.Decode(&body); err != nil {
		return endpoint, err
	}

	endpoint.TokenURL = body.AuthServer.URL + "/oauth/token"
	endpoint.AuthURL = body.AuthServer.URL + "/oauth/authorize"

	return endpoint, nil
}
