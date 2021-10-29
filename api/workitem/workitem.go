package workitem

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"tasker/ptr"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

type API struct {
	client  workitemtracking.Client
	project string
	team    string
}

type Link struct {
	URL  string
	Type string
}

func NewClient(ctx context.Context, conn *azuredevops.Connection, team, project string) (*API, error) {
	client, err := workitemtracking.NewClient(ctx, conn)
	if err != nil {
		return nil, err
	}
	return &API{
		client:  client,
		project: project,
		team:    team,
	}, nil
}

func (api *API) GetReference(ctx context.Context, taskID int) (*workitemtracking.WorkItemReference, error) {
	workItem, err := api.client.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id: ptr.FromInt(taskID),
	})
	if err != nil {
		return nil, err
	}

	if workItem == nil {
		return nil, fmt.Errorf("work item with %d not found", taskID)
	}

	return &workitemtracking.WorkItemReference{
		Id:  workItem.Id,
		Url: workItem.Url,
	}, nil
}

func (api *API) Get(ctx context.Context, taskID int) (*workitemtracking.WorkItem, error) {
	return api.client.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id: ptr.FromInt(taskID),
	})
}

func (api *API) FindCommonUserStory(ctx context.Context, currentIterationPath string) (*workitemtracking.WorkItemReference, error) {
	queryResult, err := api.client.QueryByWiql(ctx, workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: ptr.FromStr(`
				SELECT [Id], [Title]
				FROM WorkItems
				WHERE [Work Item Type] = 'User Story'
					AND [System.IterationPath] = '` + currentIterationPath + `'
					AND [Title] CONTAINS 'Общие задачи'
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
		return &(*queryResult.WorkItems)[0], nil
	}

	return nil, errors.New("active user story with name '*Общие задачи*' not found in current sprint")
}

func (api *API) Create(ctx context.Context, title, description, discipline, currentIterationPath string, estimate int, links []*Link, tags []string) (*workitemtracking.WorkItem, error) {
	tags = append(tags, "tasker")

	fields := []webapi.JsonPatchOperation{
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.IterationPath"),
			Value: currentIterationPath,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.AreaPath"),
			Value: api.project + "\\" + api.team,
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
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.Common.Discipline"),
			Value: discipline,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"),
			Value: estimate,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/Microsoft.VSTS.Scheduling.RemainingWork"),
			Value: estimate,
		},
		{
			Op:    &webapi.OperationValues.Add,
			Path:  ptr.FromStr("/fields/System.Tags"),
			Value: strings.Join(tags, "; "),
		},
	}

	for _, link := range links {
		fields = append(fields, webapi.JsonPatchOperation{
			Op:   &webapi.OperationValues.Add,
			Path: ptr.FromStr("/relations/-"),
			Value: workitemtracking.WorkItemRelation{
				Rel: ptr.FromStr(link.Type),
				Url: &link.URL,
			},
		})
	}

	task, err := api.client.CreateWorkItem(ctx, workitemtracking.CreateWorkItemArgs{
		Type:     ptr.FromStr("Task"),
		Project:  &api.project,
		Document: &fields,
	})
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (api *API) Assign(ctx context.Context, task *workitemtracking.WorkItem, user string) error {
	_, err := api.client.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
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
				Value: "InProgress",
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
