package wiki

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setNodesToNil(tasks []*Task) {
	for i := range tasks {
		tasks[i].tfsColumn = nil
		tasks[i].tr = nil
	}
}

func Test_ParseTasks_Page1(t *testing.T) {
	file, err := os.ReadFile("../testdata/wiki_page_1.html")
	assert.NoError(t, err)

	tasks, err := ParseTasksTable(string(file))
	assert.NoError(t, err)
	assert.NotEmpty(t, tasks)
	assert.Len(t, tasks, 6)

	expected := []*Task{
		{
			Title:       "Batch синхронизация пользователей",
			Description: "<p>Добавление</p><p>Удаление</p><p>Редактирование</p>",
			Estimate:    3,
		},
		{
			Title:       "Batch синхронизация атрибутов пользователя",
			Description: "Редактирование атрибутов пользователей",
			Estimate:    1,
		},
		{
			Title:       "Синхронизация пользователей по нотификациям",
			Description: "<p>Добавление</p><p>Удаление</p><p>Редактирование</p>",
			Estimate:    3,
		},
		{
			Title:       "Синхронизация атрибутов ассетов по нотификациям",
			Description: "Редактирование атрибутов пользователей",
			Estimate:    1,
		},
		{
			Title:       "Сведение",
			Description: "<br/>",
			Estimate:    6,
		},
		{
			Title:       "Демо",
			Description: "<br/>",
			Estimate:    4,
		},
	}

	setNodesToNil(tasks)
	assert.Equal(t, expected, tasks)
}

func Test_ParseTasks_Page2(t *testing.T) {
	file, err := os.ReadFile("../testdata/wiki_page_2.html")
	assert.NoError(t, err)

	tasks, err := ParseTasksTable(string(file))
	assert.NoError(t, err)
	assert.NotEmpty(t, tasks)
	assert.Len(t, tasks, 10)

	expected := []*Task{
		{
			Title:       "Репозиторий хранения классификатора",
			Description: "<p>Зфкек 1 Сохранение (Upsert) файла классификатора</p><p>Cохранение версии классификатора (для получения версии без необходимости чтения XML)</p><p>Сохранение (Upsert) файла XSD схемы</p><p>Получение классификатора</p><p>Получение версии классификатора</p><p>Получение XSD схемы</p><p>Удаление классификатора</p>",
			Estimate:    7,
			TfsTaskID:   71711,
		},
		{
			Title:       "Сервис классификатора",
			Description: "<p>Проверка по XSD схеме</p><p>Добавление в хранилище</p><p>Загрузка файла классификатора\u00a0</p><p>Загрузка файла XSD схемы</p><p>Получение версии классификатора</p><p>Получение файла классификатора\u00a0</p><p>Получение XSD файла\u00a0</p><p>Удаление классификатора</p>",
			Estimate:    7,
			TfsTaskID:   71712,
		},
		{
			Title:       "Контроллер",
			Description: "<p>Загрузка файла классификатора (HTTP POST)</p><p>Загрузка файла XSD схемы (HTTP POST)</p><p>Получение версии классификатора (HTTP GET /)</p><p>Получение файла классификатора (HTTP GET /)</p><p>Получение XSD файла (HTTP GET /)</p><p>Удаление классификатора (HTTP DELETE)</p>",
			Estimate:    6,
			TfsTaskID:   71713,
		},
		{
			Title:       "Добавление версии DPI в результирующую политику",
			Description: "<p>Добавлять версию во все политики</p><p>Если классификатора нет, то вставлять фейковую запись</p>",
			Estimate:    5,
			TfsTaskID:   71714,
		},
		{
			Title:       "События журнала действий пользователя",
			Description: "<p><span style=\"color: rgb(23,43,77);\">Загрузка DPI-классификатора</span></p><p><span style=\"color: rgb(23,43,77);\">Удаление DPI-классификатора</span></p><p><span style=\"color: rgb(23,43,77);\"><ac:emoticon ac:name=\"question\"></ac:emoticon>Загрузка XSD-схемы</span></p>",
			Estimate:    5,
			TfsTaskID:   71715,
		},
		{
			Title:       "UI. Новый раздел",
			Description: "Пункт меню, сервис для контроллера (можно пустой), заглушка страницы раздела",
			Estimate:    4,
			TfsTaskID:   71716,
		},
		{
			Title:       "UI. Отображение версии классификатора",
			Description: "<p>Метод сервиса для получения версии</p><p>Верстка</p>",
			Estimate:    4,
			TfsTaskID:   71717,
		},
		{
			Title:       "UI. Загрузка классификатора",
			Description: "<p>Метод сервиса</p><p>Верстка (вероятно кнопка с коном выбора файла)</p>",
			Estimate:    4,
			TfsTaskID:   71718,
		},
		{
			Title:       "UI. Загрузка XSD схемы",
			Description: "<p>Метод сервиса</p><p>Верстка (вероятно кнопка с коном выбора файла)</p>",
			Estimate:    4,
			TfsTaskID:   71719,
		},
		{
			Title:       "Сведение",
			Description: "<br/>",
			Estimate:    8,
			TfsTaskID:   71720,
		},
	}

	setNodesToNil(tasks)
	assert.Equal(t, expected, tasks)
}
