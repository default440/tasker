package wiki

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	goconfluence "github.com/virtomize/confluence-go-api"
)

var (
	pagesCache map[string]*goconfluence.Content = make(map[string]*goconfluence.Content)
)

type API struct {
	*goconfluence.API
}

func (a *API) MovePageNew(pageID, targetID uint) error {
	ep, err := getContentMoveEndpoint(strconv.Itoa(int(pageID)), strconv.Itoa(int(pageID)))
	if err != nil {
		return err
	}
	_, err = a.SendContentRequest(ep, "PUT", nil)
	return err
}

func (a *API) MovePageWithUpdate(pageID, targetPageID uint) error {
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

func (a *API) GetPageByID(id string) (*goconfluence.Content, error) {
	if p, ok := pagesCache[id]; ok {
		return p, nil
	}

	p, err := a.GetContentByID(id, goconfluence.ContentQuery{
		Expand: []string{
			"space",
			"version",
		},
	})

	if err == nil {
		pagesCache[p.Title] = p
		pagesCache[p.ID] = p
	}

	return p, err
}

func (a *API) GetPageByTitle(title, spaceKey string) (*goconfluence.Content, error) {
	if p, ok := pagesCache[title]; ok {
		return p, nil
	}

	sr, err := a.GetContent(goconfluence.ContentQuery{
		SpaceKey: spaceKey,
		Title:    title,
		Expand: []string{
			"space",
			"version",
		},
	})

	if err != nil {
		return nil, err
	}

	if len(sr.Results) > 1 {
		return nil, fmt.Errorf("found more than one page with title '%s' in space '%s'", title, spaceKey)
	}

	if len(sr.Results) == 0 {
		return nil, fmt.Errorf("page with title '%s' in space '%s' not found", title, spaceKey)
	}

	p := &sr.Results[0]
	pagesCache[p.Title] = p
	pagesCache[p.ID] = p

	return p, nil
}

func (a *API) UploadContent(targetPageID uint, content string, contentType string) error {
	page, err := a.GetPageByID(strconv.Itoa(int(targetPageID)))
	if err != nil {
		return err
	}

	_, err = a.UpdateContent(&goconfluence.Content{
		ID:    page.ID,
		Type:  page.Type,
		Title: page.Title,
		Body: goconfluence.Body{
			Storage: goconfluence.Storage{
				Value:          content,
				Representation: contentType,
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

func (a *API) MovePage(spaceKey, pageTitleOrID, targetPageTitleOrID string) error {
	var pageID, targetPageID string
	var page, targetPage *goconfluence.Content

	if pageTitleOrID == "" {
		return errors.New("moving page id or title invalid")
	}

	if targetPageTitleOrID == "" {
		return errors.New("target page id or title invalid")
	}

	endpoint, err := getPageMoveEndpoint()
	if err != nil {
		return err
	}

	if isID(pageTitleOrID) {
		pageID = pageTitleOrID
	} else {
		page, err = a.GetPageByTitle(pageTitleOrID, spaceKey)
		if err != nil {
			return err
		}

		pageID = page.ID
	}

	if isID(targetPageTitleOrID) {
		targetPageID = targetPageTitleOrID
	} else {
		targetPage, err = a.GetPageByTitle(targetPageTitleOrID, spaceKey)
		if err != nil {
			return err
		}
		targetPageID = targetPage.ID
	}

	if spaceKey == "" {
		if page == nil && targetPage == nil {
			page, err = a.GetPageByID(pageID)
			if err != nil {
				return err
			}
		}

		if page != nil {
			spaceKey = page.Space.Key
		} else if targetPage != nil {
			spaceKey = targetPage.Space.Key
		}
	}

	data := url.Values{}
	data.Add("position", "append")
	data.Set("spaceKey", spaceKey)
	data.Set("pageId", pageID)
	data.Add("targetId", targetPageID)

	encodedData := data.Encode()

	url := endpoint.String() + "?" + encodedData
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	_, err = a.Request(req)
	if err != nil {
		return err
	}

	return nil
}

func isID(value string) bool {
	_, err := strconv.Atoi(value)
	return err == nil
}
