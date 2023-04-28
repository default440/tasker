package wiki

import (
	"net/url"
)

func getContentMoveEndpoint(pageID, targetID string) (*url.URL, error) {
	endpoint, err := getAPIBaseAddress()
	if err != nil {
		return nil, err
	}
	return url.ParseRequestURI(endpoint + "/content/" + pageID + "/move/append/" + targetID)
}
