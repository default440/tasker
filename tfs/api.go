package tfs

import (
	"context"
	"errors"
	"tasker/tfs/connection"
	"tasker/tfs/identity"
	"tasker/tfs/work"
	"tasker/tfs/workitem"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/workitemtracking"
	"github.com/spf13/viper"
)

var (
	ErrFailedToAssign = errors.New("failed to assign task")
)

type API struct {
	Client  *workitem.Client
	Conn    *azuredevops.Connection
	project string
	team    string
}

func NewAPI(ctx context.Context) (*API, error) {
	conn := connection.Create()
	project := viper.GetString("tfsProject")
	team := viper.GetString("tfsTeam")

	client, err := workitem.NewClient(ctx, conn, team, project)
	if err != nil {
		return nil, err
	}

	return &API{
		Client:  client,
		Conn:    conn,
		project: project,
		team:    team,
	}, nil
}

func (a *API) CreateTask(ctx context.Context, title, description string, estimate float32, parentID int, relations []*workitem.Relation, tags []string, parentNamePattern string, assign bool) (*workitemtracking.WorkItem, error) {
	var err error
	var parent *workitemtracking.WorkItem
	var user string

	if assign {
		user, err = identity.Get(ctx, a.Conn)
		if err != nil {
			return nil, err
		}
	}

	if parentID > 0 {
		parent, err = a.Client.Get(ctx, parentID)
	} else {
		parent, err = a.findParent(ctx, parentNamePattern)
	}
	if err != nil {
		return nil, err
	}

	parentRelation := workitem.Relation{
		URL:  *parent.Url,
		Type: "System.LinkTypes.Hierarchy-Reverse",
	}
	relations = append(relations, &parentRelation)
	iterationPath := workitem.GetIterationPath(parent)
	areaPath := workitem.GetAreaPath(parent)

	if description == "" {
		description = title
	}

	task, err := a.Client.Create(ctx, title, description, areaPath, iterationPath, estimate, relations, tags)
	if err != nil {
		return nil, err
	}

	if assign {
		err = a.Client.Assign(ctx, task, user)
		if err != nil {
			return task, err
		}
	}

	return task, nil
}

func (a *API) findParent(ctx context.Context, namePattern string) (*workitemtracking.WorkItem, error) {
	iterations, err := work.GetIterations(ctx, a.Conn, a.project, a.team)
	if err != nil {
		return nil, err
	}

	for i := len(*iterations) - 1; i >= 0; i-- {
		iteration := (*iterations)[i]
		if *iteration.Attributes.TimeFrame == "current" || *iteration.Attributes.TimeFrame == "past" {
			userStory, err := a.Client.FindRequirement(ctx, namePattern, *iteration.Path)
			if err != nil {
				return nil, err
			}
			if userStory != nil {
				return userStory, nil
			}
		}
	}

	return nil, errors.New("active user story with name contains '" + namePattern + "' not found in current and previous sprints")
}

func (a *API) CreateFeatureTask(ctx context.Context, title, description string, estimate float32, feature *workitemtracking.WorkItem, tags []string) (*workitemtracking.WorkItem, error) {
	iterationPath := workitem.GetIterationPath(feature)
	areaPath := workitem.GetAreaPath(feature)
	relations := []*workitem.Relation{
		{
			URL:  *feature.Url,
			Type: "System.LinkTypes.Hierarchy-Reverse",
		},
	}

	return a.Client.Create(ctx, title, description, areaPath, iterationPath, estimate, relations, tags)
}
