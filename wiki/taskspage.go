package wiki

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"tasker/tasksui"

	"github.com/PuerkitoBio/goquery"
	"github.com/samber/lo"
)

var (
	tasksRegexp    = regexp.MustCompile("(?i).*задач.*")
	spacesRegexp   = regexp.MustCompile(`\s+(<)|(>)\s+`)
	tablesRegexp   = regexp.MustCompile(`(?i)\<table(.|\s)+?\</table>`)
	emoticonRegexp = regexp.MustCompile(`(?i)(\<ac\:emoticon(.|\s)*?)\/\>`)
	cdataRegexp    = regexp.MustCompile(`(?i)(<!--\[CDATA\[((.|\s)*?)\]\]-->)`)
	columnsMapping = make(map[int]taskColumn)
)

type taskColumn int

const (
	titleColumn taskColumn = iota
	descColumn
	estColumn
	tfsColumn
	tagsColumn
)

type Task struct {
	Title       string
	Description string
	Estimate    float32
	TfsTaskID   int
	Tags        []string
	tfsColumn   *goquery.Selection
	updated     bool
	tr          *goquery.Selection
}

func (t *Task) GetTitle() string                  { return t.Title }
func (t *Task) SetTitle(title string)             { t.Title = title }
func (t *Task) GetDescription() string            { return t.Description }
func (t *Task) SetDescription(description string) { t.Description = description }
func (t *Task) GetEstimate() float32              { return t.Estimate }
func (t *Task) SetEstimate(estimate float32)      { t.Estimate = estimate }
func (t *Task) GetTfsTaskID() int                 { return t.TfsTaskID }
func (t *Task) SetTfsTaskID(tfsTaskID int)        { t.TfsTaskID = tfsTaskID }
func (t *Task) Clone() tasksui.Task {
	t2 := *t
	return &t2
}
func (t *Task) GetTags() []string     { return t.Tags }
func (t *Task) SetTags(tags []string) { t.Tags = tags }
func (t *Task) GetTagsString() string { return strings.Join(t.Tags, "; ") }
func (t *Task) SetTagsString(tagsString string) {
	if len(strings.TrimSpace(tagsString)) > 0 {
		t.Tags = strings.Split(tagsString, "; ")
	} else {
		t.Tags = nil
	}
}

type Table struct {
	Number int
	Index  int
	Tasks  []*Task
}

func (t *Table) GetTasks() []tasksui.Task {
	return lo.Map(t.Tasks, func(tsk *Task, _ int) tasksui.Task { return tsk })
}
func (t *Table) SetTask(tsk tasksui.Task, index int) {
	t.Tasks[index] = tsk.(*Task)
}

func (t *Task) Update(html string) {
	if t.tfsColumn != nil {
		t.tfsColumn.SetHtml(html)
		t.updated = true
	}
}

func (t *Task) isEmpty() bool {
	return t.Estimate == 0 && t.TfsTaskID == 0
}

func (t *Task) TableIndex() int {
	table := t.Table()
	indexStr, _ := table.Attr("index")
	index, err := strconv.Atoi(indexStr)
	if err == nil {
		return index
	}

	return -1
}

func (t *Task) Table() *goquery.Selection {
	return t.tr.Parent().Parent()
}

func ParseTasksTable(body string) ([]*Task, error) {
	body = fixMarkup(body)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	var tasks []*Task

	_ = doc.Find("table").
		Each(func(i int, s *goquery.Selection) {
			if len(s.Find("table").Nodes) == 0 {
				text := "&lt;code&gt;"

				s.Find("code").Each(func(i int, ss *goquery.Selection) {
					code := ss.Text()
					code = strings.Replace(code, "<", "&lt;", -1)
					code = strings.Replace(code, ">", "&gt;", -1)
					text = text + "<p>" + code + "</p>"
				})

				text = text + "&lt;/code&gt;"

				s.ReplaceWithHtml("<div>" + removeExtraSpaces(text) + "</div>")
			}
		}).
		FilterFunction(func(i int, s *goquery.Selection) bool {
			return tasksRegexp.MatchString(s.Prev().Text())
		}).
		Each(func(i int, s *goquery.Selection) {
			s.SetAttr("index", fmt.Sprintf("%d", i))
		}).
		Find("tr").
		Each(func(i int, tr *goquery.Selection) {
			tr.Find("th").Each(func(colNum int, th *goquery.Selection) {
				switch strings.ToLower(th.Text()) {
				case "задача":
					columnsMapping[colNum] = titleColumn
				case "описание":
					columnsMapping[colNum] = descColumn
				case "оценка":
					columnsMapping[colNum] = estColumn
				case "tfs":
					columnsMapping[colNum] = tfsColumn
				case "тег":
				case "теги":
					columnsMapping[colNum] = tagsColumn
				}
			})

			cols := tr.Find("td")
			if len(cols.Nodes) < len(columnsMapping) {
				return
			}

			task := &Task{
				tr: tr,
			}
			cols.Each(func(colNum int, td *goquery.Selection) {
				column, ok := columnsMapping[colNum]
				if ok {
					switch column {
					case titleColumn:
						title := td.Text()
						task.Title = strings.TrimSpace(title)
					case descColumn:
						html, _ := td.Html()
						task.Description = removeExtraSpaces(html)
					case estColumn:
						floatValue, _ := strconv.ParseFloat(strings.TrimSpace(td.Text()), 32)
						task.Estimate = float32(floatValue)
					case tfsColumn:
						task.TfsTaskID = parseTfsTaskID(td)
						task.tfsColumn = td
					case tagsColumn:
						tagsStr := td.Text()
						tags := regexp.MustCompile(`[\PL]`).Split(tagsStr, -1)
						tags = slices.DeleteFunc(tags, func(s string) bool { return s == "" })
						task.Tags = tags
					}
				}
			})

			if !task.isEmpty() && task.TfsTaskID != -1 {
				tasks = append(tasks, task)
			}
		})

	return tasks, nil
}

func removeExtraSpaces(value string) string {
	return strings.TrimSpace(spacesRegexp.ReplaceAllString(value, "$1$2"))
}

func parseTfsTaskID(td *goquery.Selection) int {
	text := td.Find("ac\\:parameter[ac\\:name='itemID']").Text()
	taskID, err := strconv.Atoi(text)
	if err != nil {
		if len(td.Text()) > 0 {
			return -1
		}
		return 0
	}
	return taskID
}

func UpdatePageContent(body string, tasks []*Task) (string, bool, error) {
	updatedTasks := getUpdatedTasks(tasks)
	if len(updatedTasks) == 0 {
		return "", false, nil
	}

	var htmlTables []string
	htmlTableIndex := 0
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(body))

	doc.Find("table").
		Each(func(i int, s *goquery.Selection) {
			if len(s.Find("table").Nodes) == 0 {
				htmlTable, _ := s.Html()
				htmlTables = append(htmlTables, "<table class=\"wrapped\" data-mce-resize=\"false\" style=\"text-align: left;\">"+htmlTable+"</table>")
				s.ReplaceWithHtml("<meta>link " + strconv.Itoa(htmlTableIndex) + "</meta>")
				htmlTableIndex = htmlTableIndex + 1
			}
		})

	html, _ := doc.Find("body").Html()

	body = html

	tables := make(map[int]*goquery.Selection)
	for _, t := range tasks {
		tableIndex := t.TableIndex()

		if tableIndex == -1 {
			return "", false, errors.New("table index not found")
		}

		if _, ok := tables[tableIndex]; !ok {
			tables[tableIndex] = t.Table()
		}
	}

	tablesIndexes := tablesRegexp.FindAllStringIndex(body, -1)
	maxIndex := getMaxIndex(tables)
	if maxIndex >= len(tablesIndexes) {
		return "", false, errors.New("not all tasks tables found")
	}

	for index, table := range tables {
		tableElement := table.Clone()
		tableElement.Empty()
		tableElement.RemoveAttr("index")
		table.WrapInnerSelection(tableElement)
		tableMarkup, _ := table.Html()
		tableMarkup = restoreMarkup(tableMarkup)

		i := tablesIndexes[index]
		modifiedBody := body[:i[0]]
		modifiedBody += tableMarkup
		modifiedBody += body[i[1]:]
		body = modifiedBody
		tablesIndexes = tablesRegexp.FindAllStringIndex(body, -1)
	}

	for i := 0; i < len(htmlTables); i++ {
		begin := strings.Index(body, "&lt;code&gt;")
		end := strings.Index(body, "&lt;/code&gt;")

		body = body[:begin] + htmlTables[i] + body[end+13:]
	}

	return body, true, nil
}

func getMaxIndex(tables map[int]*goquery.Selection) int {
	var maxIndex int
	for index := range tables {
		if index > maxIndex {
			maxIndex = index
		}
	}
	return maxIndex
}

func getUpdatedTasks(tasks []*Task) []*Task {
	var updated []*Task
	for _, v := range tasks {
		if v.updated {
			updated = append(updated, v)
		}
	}
	return updated
}

func fixMarkup(markup string) string {
	markup = emoticonRegexp.ReplaceAllString(markup, "$1></ac:emoticon>")
	return markup
}

func restoreMarkup(markup string) string {
	markup = cdataRegexp.ReplaceAllString(markup, "<![CDATA[$2]]>")
	return markup
}

func GroupByTable(tasks []*Task) ([]*Table, error) {
	var tables []*Table
	m := make(map[int]*Table)
	for _, t := range tasks {
		tableIndex := t.TableIndex()

		if tableIndex == -1 {
			return nil, errors.New("table index not found")
		}

		table, ok := m[tableIndex]
		if !ok {
			table = &Table{
				Number: len(tables) + 1,
				Index:  tableIndex,
			}
			tables = append(tables, table)
			m[tableIndex] = table
		}

		table.Tasks = append(table.Tasks, t)
	}

	return tables, nil
}
