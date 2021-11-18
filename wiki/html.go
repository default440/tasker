package wiki

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	tasksRegexp    = regexp.MustCompile("(?i).*задач.*")
	spacesRegexp   = regexp.MustCompile(`\s+(<)|(>)\s+`)
	columnsMapping = make(map[int]taskColumn)
)

type taskColumn int

const (
	titleColumn taskColumn = iota
	descColumn
	estColumn
	tfsColumn
)

type Task struct {
	Title       string
	Description string
	Estimate    float32
	TfsTaskID   int
	TfsColumn   *goquery.Selection
}

func (t *Task) isEmpty() bool {
	return t.Title == "" &&
		t.Description == "" &&
		t.Estimate == 0 &&
		t.TfsTaskID == 0
}

func ParseTasks(body string) ([]Task, *goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, nil, err
	}

	var tasks []Task

	_ = doc.Find("table").
		FilterFunction(func(i int, s *goquery.Selection) bool {
			return tasksRegexp.MatchString(s.Prev().Text())
		}).
		Find("tr").
		Each(func(i int, s *goquery.Selection) {
			s.Find("th").Each(func(colNum int, th *goquery.Selection) {
				switch strings.ToLower(th.Text()) {
				case "задача":
					columnsMapping[colNum] = titleColumn
				case "описание":
					columnsMapping[colNum] = descColumn
				case "оценка":
					columnsMapping[colNum] = estColumn
				case "tfs":
					columnsMapping[colNum] = tfsColumn
				}
			})

			cols := s.Find("td")
			if len(cols.Nodes) < len(columnsMapping) {
				return
			}

			task := Task{}
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
						task.TfsColumn = td
					}
				}
			})

			if !task.isEmpty() && task.TfsTaskID != -1 {
				tasks = append(tasks, task)
			}
		})

	return tasks, doc, nil
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
