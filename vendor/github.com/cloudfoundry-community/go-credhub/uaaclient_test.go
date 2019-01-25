package credhub_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	uaa "code.cloudfoundry.org/uaa-go-client"
	"code.cloudfoundry.org/uaa-go-client/config"
	credhub "github.com/cloudfoundry-community/go-credhub"
)

func TestUAAAuthedClient_Get(t *testing.T) {
	ts := uaaTestServer()
	defer ts.Close()

	uaaAuthClient := testUAAAuthClient(t, ts.URL)
	_, err := uaaAuthClient.Get("http://does.not.matter.com")
	if err != nil {
		t.Fatal(err)
	}
}

func TestUAAAuthedClient_Do(t *testing.T) {
	ts := uaaTestServer()
	defer ts.Close()

	uaaAuthClient := testUAAAuthClient(t, ts.URL)

	req, err := http.NewRequest("GET", "http://does.not.matter.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = uaaAuthClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
}

type fakeClient struct{}

func (c *fakeClient) Get(url string) (resp *http.Response, err error) { return nil, nil }
func (c *fakeClient) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("authorization") != "bearer 8d952f1311c041d19253fc01c2145144" {
		return nil, errors.New("expected correct bearer token")
	}
	return nil, nil
}

func uaaTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			fmt.Fprintf(w, `{
			  "access_token" : "8d952f1311c041d19253fc01c2145144",
			  "token_type" : "bearer",
			  "id_token" : "eyJhbGciOiJIUzI1NiIsImprdSI6Imh0dHBzOi8vbG9jYWxob3N0OjgwODAvdWFhL3Rva2VuX2tleXMiLCJraWQiOiJsZWdhY3ktdG9rZW4ta2V5IiwidHlwIjoiSldUIn0.eyJzdWIiOiJjMWJhZTk2OC1hMjFlLTQ5ODItOGQwYi03ODJjMjQwNGI3OWYiLCJhdWQiOlsibG9naW4iXSwiaXNzIjoiaHR0cDovL2xvY2FsaG9zdDo4MDgwL3VhYS9vYXV0aC90b2tlbiIsImV4cCI6MTU0NTQ3NjcwNSwiaWF0IjoxNTQ1NDMzNTA1LCJhenAiOiJsb2dpbiIsInNjb3BlIjpbIm9wZW5pZCJdLCJlbWFpbCI6IkQ3a1J6RkB0ZXN0Lm9yZyIsInppZCI6InVhYSIsIm9yaWdpbiI6InVhYSIsImp0aSI6IjhkOTUyZjEzMTFjMDQxZDE5MjUzZmMwMWMyMTQ1MTQ0IiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImNsaWVudF9pZCI6ImxvZ2luIiwiY2lkIjoibG9naW4iLCJncmFudF90eXBlIjoiYXV0aG9yaXphdGlvbl9jb2RlIiwidXNlcl9uYW1lIjoiRDdrUnpGQHRlc3Qub3JnIiwicmV2X3NpZyI6IjRkOWQ4ZjY5IiwidXNlcl9pZCI6ImMxYmFlOTY4LWEyMWUtNDk4Mi04ZDBiLTc4MmMyNDA0Yjc5ZiIsImF1dGhfdGltZSI6MTU0NTQzMzUwNX0.DDqZtEIaTgtIhT0iaRyEoNvDpsGvHuUMyxOS9Zo5fhI",
			  "refresh_token" : "331e025fe0384bf588fae5bba0b7f784-r",
			  "expires_in" : 43199,
			  "scope" : "openid oauth.approvals",
			  "jti" : "8d952f1311c041d19253fc01c2145144"
			}`)
		}
	}))
}

func testUAAAuthClient(t *testing.T, url string) credhub.HTTPClient {
	cfg := &config.Config{
		ClientName:       "client-name",
		ClientSecret:     "client-secret",
		UaaEndpoint:      url,
		SkipVerification: true,
	}

	clock := fakeclock.NewFakeClock(time.Now())
	logger := lagertest.NewTestLogger("test")
	uaaClient, err := uaa.NewClient(logger, cfg, clock)
	if err != nil {
		t.Fatal(err)
	}

	fake := &fakeClient{}
	return credhub.NewUAAAuthClient(fake, uaaClient)
}
