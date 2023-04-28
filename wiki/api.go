package wiki

import (
	"strconv"

	goconfluence "github.com/virtomize/confluence-go-api"
)

func MovePage(a *goconfluence.API, pageID, targetID uint) error {
	ep, err := getContentMoveEndpoint(strconv.Itoa(int(pageID)), strconv.Itoa(int(pageID)))
	if err != nil {
		return err
	}
	_, err = a.SendContentRequest(ep, "PUT", nil)
	return err
}
