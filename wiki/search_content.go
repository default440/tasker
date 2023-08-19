package wiki

import (
	"net/url"
	"strconv"
	"strings"

	goconfluence "github.com/virtomize/confluence-go-api"
)

// getContentEndpoint creates the correct api endpoint by given id
func (a *API) getSearchContntEndpoint() (*url.URL, error) {
	return url.ParseRequestURI(a.endPoint.String() + "/content/search")
}

// Search querys confluence using CQL
func (a *API) SearchContent(query goconfluence.SearchQuery) (*goconfluence.Search, error) {
	ep, err := a.getSearchContntEndpoint()
	if err != nil {
		return nil, err
	}
	ep.RawQuery = addSearchQueryParams(query).Encode()
	return a.SendSearchRequest(ep, "GET")
}

// addSearchQueryParams adds the defined query parameters
func addSearchQueryParams(query goconfluence.SearchQuery) *url.Values {

	data := url.Values{}
	if query.CQL != "" {
		data.Set("cql", query.CQL)
	}
	if query.CQLContext != "" {
		data.Set("cqlcontext", query.CQLContext)
	}
	if query.IncludeArchivedSpaces {
		data.Set("includeArchivedSpaces", "true")
	}
	if query.Limit != 0 {
		data.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Start != 0 {
		data.Set("start", strconv.Itoa(query.Start))
	}
	if len(query.Expand) != 0 {
		data.Set("expand", strings.Join(query.Expand, ","))
	}
	return &data
}
