package wiki

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"

	"github.com/spf13/viper"
	goconfluence "github.com/virtomize/confluence-go-api"
)

func NewClient() (*goconfluence.API, error) {
	accessToken := viper.GetString("wikiAccessToken")
	username := viper.GetString("wikiUserName")
	password := viper.GetString("wikiPassword")

	apiBaseAddress, err := getAPIBaseAddress()
	if err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &addAuthHeaderRoundTripper{
			accessToken: accessToken,
			username:    username,
			password:    password,
			inner: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		Jar: jar,
	}

	//goconfluence.DebugFlag = true

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

func getContentMoveEndpoint(pageID, targetID string) (*url.URL, error) {
	endpoint, err := getAPIBaseAddress()
	if err != nil {
		return nil, err
	}
	return url.ParseRequestURI(endpoint + "/content/" + pageID + "/move/append/" + targetID)
}

func getPageMoveEndpoint() (*url.URL, error) {
	baseAddress := viper.GetString("wikiBaseAddress")
	return url.ParseRequestURI(baseAddress + "/pages/movepage.action")
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
	username    string
	password    string
}

func (rt *addAuthHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+rt.accessToken)
	} else if rt.username != "" && rt.password != "" {
		req.Header.Add("Authorization", "Basic "+basicAuth(rt.username, rt.password))
	}

	req.Header.Set("X-Atlassian-Token", "no-check")

	return rt.inner.RoundTrip(req)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
