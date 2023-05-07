package wiki

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"tasker/prettyprint"

	goconfluence "github.com/virtomize/confluence-go-api"
)

func MovePageNew(a *goconfluence.API, pageID, targetID uint) error {
	ep, err := getContentMoveEndpoint(strconv.Itoa(int(pageID)), strconv.Itoa(int(pageID)))
	if err != nil {
		return err
	}
	_, err = a.SendContentRequest(ep, "PUT", nil)
	return err
}

func MovePageWithUpdate(a *goconfluence.API, pageID, targetPageID uint) error {
	var err error
	getPage := func(id uint) (*goconfluence.Content, error) {
		return a.GetContentByID(strconv.Itoa(int(id)), goconfluence.ContentQuery{
			Expand: []string{
				"body.storage",
				"space",
				"version",
			},
		})
	}

	page, err := getPage(pageID)
	if err != nil {
		return err
	}

	_, err = a.UpdateContent(&goconfluence.Content{
		ID:    page.ID,
		Type:  page.Type,
		Title: page.Title,
		Ancestors: []goconfluence.Ancestor{
			{ID: strconv.Itoa(int(targetPageID))},
		},
		Body: goconfluence.Body{
			Storage: goconfluence.Storage{
				Value:          page.Body.Storage.Value,
				Representation: "storage",
			},
		},
		Space: &goconfluence.Space{
			Key: page.Space.Key,
		},
		Version: &goconfluence.Version{
			Number: page.Version.Number + 1,
		},
	})

	return err
}

func MovePage(a *goconfluence.API, pageID, targetPageID uint) (string, error) {
	endpoint, err := getPageMoveEndpoint()
	if err != nil {
		return "", err
	}

	getPage := func(id uint) (*goconfluence.Content, error) {
		return a.GetContentByID(strconv.Itoa(int(id)), goconfluence.ContentQuery{
			Expand: []string{
				//"body.storage",
				"space",
				"version",
			},
		})
	}

	page, err := getPage(pageID)
	if err != nil {
		return "", err
	}

	data := url.Values{}
	data.Set("spaceKey", page.Space.Key)
	data.Set("pageId", page.ID)
	data.Add("targetId", strconv.Itoa(int(targetPageID)))
	data.Add("position", "append")

	encodedData := data.Encode()

	url := endpoint.String() + "?" + encodedData
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}

	res, err := a.Request(req)
	if err != nil {
		return "", err
	}

	var content goconfluence.Content
	if len(res) != 0 {
		err = json.Unmarshal(res, &content)
		if err != nil {
			return "", err
		}

		prettyprint.JSONObjectColor(content)
	}

	return page.Title, err
}
