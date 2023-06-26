package wiki

import (
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	goconfluence "github.com/virtomize/confluence-go-api"
)

var (
	tfsTaskStructuredMacroAttributes = map[string]func(value string, t *TfsTask) error{
		"itemID": func(value string, t *TfsTask) error {
			parsedValue, err := strconv.Atoi(value)
			if err != nil {
				return err
			}

			t.ItemID = parsedValue
			return nil
		},
	}
)

type TfsTask struct {
	ItemID int
}

type TechDebt struct {
	PageID      string
	Title       string
	Description string
	Body        string
	IsEmptyPage bool
	Labels      []string
	TfsTasks    []TfsTask
}

func (td *TechDebt) GetUpdatedBody() (string, error) {
	return td.Body, nil
}

func (td *TechDebt) AddTfsTask(id int) {
	html := `<p>
	<ac:structured-macro ac:name="work-item-tfs" ac:schema-version="1" ac:macro-id="` + uuid.NewString() + `">
		<ac:parameter ac:name="itemID">` + strconv.Itoa(id) + `</ac:parameter>
		<ac:parameter ac:name="host">1</ac:parameter>
		<ac:parameter ac:name="assigned">true</ac:parameter>					
		<ac:parameter ac:name="status">true</ac:parameter>
	</ac:structured-macro>
</p>
`

	td.Body = html + td.Body
	td.TfsTasks = append(td.TfsTasks, TfsTask{
		ItemID: id,
	})
}

func ParseTechDebt(content *goconfluence.Content) (TechDebt, error) {
	body := content.Body.Storage.Value
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return TechDebt{}, err
	}

	tasks, err := parseTfsTask(doc)
	if err != nil {
		return TechDebt{}, err
	}

	linkToPage := getWikiPageLink(content)
	description := linkToPage + body

	return TechDebt{
		PageID:      content.ID,
		Title:       strings.TrimSpace(content.Title),
		Description: description,
		Body:        body,
		TfsTasks:    tasks,
		IsEmptyPage: strings.TrimSpace(doc.Text()) == "",
	}, nil
}

func getWikiPageLink(content *goconfluence.Content) string {
	url := content.Links.Base + content.Links.WebUI
	return `<div><a href="` + url + `">` + content.Title + `</a><br></div>`
}

func parseTfsTask(doc *goquery.Document) ([]TfsTask, error) {
	var parseErr error
	var tasks []TfsTask
	doc.Find("ac\\:structured-macro[ac\\:name='work-item-tfs']").
		Each(func(i int, sm *goquery.Selection) {
			task := TfsTask{}
			sm.Find("ac\\:parameter").
				Each(func(i int, p *goquery.Selection) {
					if attrName, exists := p.Attr("ac:name"); exists {
						if parseAttr, ok := tfsTaskStructuredMacroAttributes[attrName]; ok {
							err := parseAttr(p.Text(), &task)
							if err != nil {
								parseErr = err
							}
						}
					}
				})
			tasks = append(tasks, task)
		})

	return tasks, parseErr
}
