package workitem

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"tasker/ptr"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/workitemtracking"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
)

type Client struct {
	workitemtracking.Client
	project string
	team    string
}

type Relation struct {
	URL  string
	Type string
}

type Field struct {
	Path  *string
	Value interface{}
}

func NewClient(ctx context.Context, conn *azuredevops.Connection, team, project string) (*Client, error) {
	client, err := workitemtracking.NewClient(ctx, conn)
	if err != nil {
		return nil, err
	}

	return &Client{
		client,
		project,
		team,
	}, nil
}

func (api *Client) Update(ctx context.Context, taskID int, title, description string, estimate float32) error {
	fields := []webapi.JsonPatchOperation{
		{
			Op:    &webapi.OperationValues.Replace,
			Path:  ptr.FromStr("/fields/System.Title"),
			Value: title,
		},
		{
			Op:    &webapi.OperationValues.Replace,
			Path:  ptr.FromStr("/fields/System.Description"),
			Value: description,
		},
	}
	_, err := api.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
		Id:       ptr.FromInt(taskID),
		Project:  &api.project,
		Document: &fields,
	})

	return err
}

func (api *Client) Get(ctx context.Context, workItemID int) (*workitemtracking.WorkItem, error) {
	return api.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id: ptr.FromInt(workItemID),
	})
}

func (api *Client) GetExpanded(ctx context.Context, workItemID int) (*workitemtracking.WorkItem, error) {
	return api.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:     ptr.FromInt(workItemID),
		Expand: &workitemtracking.WorkItemExpandValues.All,
	})
}

func (api *Client) Delete(ctx context.Context, workItemID int) error {
	_, err := api.DeleteWorkItem(ctx, workitemtracking.DeleteWorkItemArgs{
		Project: &api.project,
		Destroy: ptr.FromBool(true),
		Id:      ptr.FromInt(workItemID),
	})

	return err
}

func (api *Client) FindUserStory(ctx context.Context, namePattern, iterationPath string) (*workitemtracking.WorkItem, error) {
	if namePattern == "" {
		return nil, errors.New("user story name pattern is empty")
	}

	queryResult, err := api.QueryByWiql(ctx, workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: ptr.FromStr(`
				SELECT [Id], [Title], [System.AreaPath], [System.IterationPath]
				FROM WorkItems
				WHERE [Work Item Type] = 'User Story'
					AND [System.IterationPath] = '` + iterationPath + `'
					AND [Title] CONTAINS '` + namePattern + `'
					AND [State] = 'Active'
			`),
		},
		Project: &api.project,
		Team:    &api.team,
	})
	if err != nil {
		return nil, err
	}

	if len(*queryResult.WorkItems) > 0 {
		userStory := &(*queryResult.WorkItems)[0]
		return api.Get(ctx, *userStory.Id)
	}

	return nil, nil
}

func (api *Client) FindRequirement(ctx context.Context, namePattern, iterationPath, state string) (*workitemtracking.WorkItem, error) {
	if namePattern == "" {
		return nil, errors.New("user story name pattern is empty")
	}

	var iterationFilter string
	if iterationPath != "" {
		iterationFilter = `AND [System.IterationPath] = '` + iterationPath + `'`
	}

	var stateFilter string
	if state != "" {
		stateFilter = `AND [State] = '` + stateFilter + `'`
	}

	queryResult, err := api.QueryByWiql(ctx, workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: ptr.FromStr(`
				SELECT [Id], [Title], [System.AreaPath], [System.IterationPath]
				FROM WorkItems
				WHERE [Work Item Type] = 'Requirement'
					AND [Title] CONTAINS WORDS '` + namePattern + `'
					` + iterationFilter + `
					` + stateFilter + `
			`),
		},
		Project: &api.project,
		Team:    &api.team,
	})
	if err != nil {
		return nil, err
	}

	if len(*queryResult.WorkItems) > 0 {
		userStory := &(*queryResult.WorkItems)[0]
		return api.Get(ctx, *userStory.Id)
	}

	return nil, nil
}

func (api *Client) CreateRequirement(ctx context.Context, requirementType, title, description, areaPath, iterationPath string, estimate, priority float32, relations []*Relation, tags []string) (*workitemtracking.WorkItem, error) {
	fields := []*Field{
		{
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.CMMI.RequirementType"),
			Value: requirementType,
		},
		{
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.Common.ValueArea"),
			Value: "Architectural",
		},
		{
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.Common.Priority"),
			Value: fmt.Sprintf("%v", priority),
		},
		{
			Path:  ptr.FromStr("/fields/System.AssignedTo"),
			Value: nil,
		},
	}

	return api.create(ctx, "Requirement", title, description, areaPath, iterationPath, estimate, relations, fields, tags)
}

func (api *Client) CreateTask(ctx context.Context, title, description, areaPath, iterationPath string, estimate float32, relations []*Relation, tags []string) (*workitemtracking.WorkItem, error) {
	discipline := viper.GetString("tfsDiscipline")
	fields := []*Field{
		{
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.Common.Discipline"),
			Value: discipline,
		},
		{
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.Scheduling.RemainingWork"),
			Value: estimate,
		},
	}

	return api.create(ctx, "Task", title, description, areaPath, iterationPath, estimate, relations, fields, tags)
}

func (api *Client) create(ctx context.Context, workitemType, title, description, areaPath, iterationPath string, estimate float32, relations []*Relation, fields []*Field, tags []string) (*workitemtracking.WorkItem, error) {
	documentFields := []webapi.JsonPatchOperation{
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.IterationPath"),
			Value: iterationPath,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.AreaPath"),
			Value: areaPath,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.Title"),
			Value: title,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.Description"),
			Value: description,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"),
			Value: estimate,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.Tags"),
			Value: strings.Join(tags, "; "),
		},
	}

	for _, field := range fields {
		documentFields = append(documentFields, webapi.JsonPatchOperation{
			Op:    &webapi.OperationValues.Add,
			Path:  field.Path,
			Value: field.Value,
		})
	}

	for _, relation := range relations {
		documentFields = append(documentFields, webapi.JsonPatchOperation{
			Op:   &webapi.OperationValues.Add,
			Path: ptr.FromStr("/relations/-"),
			Value: workitemtracking.WorkItemRelation{
				Rel: ptr.FromStr(relation.Type),
				Url: &relation.URL,
			},
		})
	}

	task, err := api.CreateWorkItem(ctx, workitemtracking.CreateWorkItemArgs{
		Type:     ptr.From(workitemType),
		Project:  &api.project,
		Document: &documentFields,
	})
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (api *Client) Copy(ctx context.Context, sourceWorkItem *workitemtracking.WorkItem, areaPath, iterationPath string, relations []*Relation, tags []string) (*workitemtracking.WorkItem, error) {
	fields := []webapi.JsonPatchOperation{
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.IterationPath"),
			Value: iterationPath,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.AreaPath"),
			Value: areaPath,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.Tags"),
			Value: strings.Join(tags, "; "),
		},
	}

	for key, fieldValue := range *sourceWorkItem.Fields {
		if strings.Contains(key, "BoardColumn") {
			continue
		}

		if value, ok := fieldValue.(string); ok {
			path := "/fields/" + key

			if slices.ContainsFunc(fields, func(f webapi.JsonPatchOperation) bool {
				return *f.Path == path
			}) {
				continue
			}

			fields = append(fields, webapi.JsonPatchOperation{
				Op:    &webapi.OperationValues.Add,
				Path:  &path,
				Value: value,
			})
		}
	}

	for _, relation := range relations {
		fields = append(fields, webapi.JsonPatchOperation{
			Op:   &webapi.OperationValues.Add,
			Path: ptr.FromStr("/relations/-"),
			Value: workitemtracking.WorkItemRelation{
				Rel: ptr.FromStr(relation.Type),
				Url: &relation.URL,
			},
		})
	}

	workItemType := GetType(sourceWorkItem)
	task, err := api.CreateWorkItem(ctx, workitemtracking.CreateWorkItemArgs{
		Type:     &workItemType,
		Project:  &api.project,
		Document: &fields,
	})
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (api *Client) Assign(ctx context.Context, task *workitemtracking.WorkItem, user string) error {
	_, err := api.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
		Id:      task.Id,
		Project: &api.project,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:    &webapi.OperationValues.Test,
				Path:  ptr.FromStr("/rev"),
				Value: task.Rev,
			},
			{
				Op:    &webapi.OperationValues.Add,
				Path:  ptr.FromStr("/fields/System.AssignedTo"),
				Value: user,
			},
			{
				Op:    &webapi.OperationValues.Add,
				Path:  ptr.FromStr("/fields/System.State"),
				Value: "Active",
			},
		},
	})

	return err
}

func GetURL(w *workitemtracking.WorkItem) string {
	lm, ok := w.Links.(map[string]interface{})
	if ok {
		pm, ok := lm["html"].(map[string]interface{})
		if ok {
			href, ok := pm["href"]
			if ok {
				str, ok := href.(string)
				if ok {
					return str
				}
			}
		}
	}
	return *w.Url
}

func GetTitle(w *workitemtracking.WorkItem) string {
	title, ok := (*w.Fields)["System.Title"]
	if ok {
		titleStr, ok := title.(string)
		if ok {
			return titleStr
		}
	}
	return ""
}

func GetIterationPath(w *workitemtracking.WorkItem) string {
	iterationPath, ok := (*w.Fields)["System.IterationPath"]
	if ok {
		iterationPathStr, ok := iterationPath.(string)
		if ok {
			return iterationPathStr
		}
	}
	return ""
}

func GetAreaPath(w *workitemtracking.WorkItem) string {
	areaPath, ok := (*w.Fields)["System.AreaPath"]
	if ok {
		areaPathStr, ok := areaPath.(string)
		if ok {
			return areaPathStr
		}
	}
	return ""
}

func GetType(w *workitemtracking.WorkItem) string {
	workItemType, ok := (*w.Fields)["System.WorkItemType"]
	if ok {
		workItemTypeStr, ok := workItemType.(string)
		if ok {
			return workItemTypeStr
		}
	}
	return ""
}

func GetTags(w *workitemtracking.WorkItem) []string {
	tagsValue, ok := (*w.Fields)["System.Tags"]
	if ok {
		tagsStr, ok := tagsValue.(string)
		if ok {
			tags := strings.Split(tagsStr, ";")
			for i := 0; i < len(tags); i++ {
				tags[i] = strings.TrimSpace(tags[i])
			}
			return tags
		}
	}
	return nil
}
