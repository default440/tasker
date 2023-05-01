package wiki

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"path"

	"github.com/spf13/viper"
	goconfluence "github.com/virtomize/confluence-go-api"
)

func NewClient() (*goconfluence.API, error) {
	accessToken := viper.GetString("wikiAccessToken")

	apiBaseAddress, err := getAPIBaseAddress()
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &addAuthHeaderRoundTripper{
			accessToken: accessToken,
			inner: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	// goconfluence.DebugFlag = true

	return goconfluence.NewAPIWithClient(apiBaseAddress, client)
}

func getAPIBaseAddress() (string, error) {
	baseAddress := viper.GetString("wikiBaseAddress")

	wikiAddress, err := joinURL(baseAddress, "rest/api")
	if err != nil {
		return "", err
	}
	return wikiAddress, nil
}

func joinURL(base, relPath string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	u.Path = path.Join(u.Path, relPath)
	return u.String(), nil
}

type addAuthHeaderRoundTripper struct {
	inner       http.RoundTripper
	accessToken string
}

func (rt *addAuthHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+rt.accessToken)
	return rt.inner.RoundTrip(req)
}
