package tfs

import (
	"context"
	"errors"
	"fmt"
	"tasker/browser"
	"tasker/tfs/connection"
	"tasker/tfs/identity"
	"tasker/tfs/work"
	"tasker/tfs/workitem"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
	"github.com/spf13/viper"
)

var (
	ErrFailedToAssign = errors.New("failed to assign task")
)

type API struct {
	Client  *workitem.Client
	conn    *azuredevops.Connection
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
		conn:    conn,
		project: project,
		team:    team,
	}, nil
}

func (a *API) CreateTask(ctx context.Context, title, description string, estimate, parentID int, relations []*workitem.Relation, tags []string, openBrowser bool) error {
	var err error
	var iterationPath string
	var parent *workitemtracking.WorkItemReference

	user, err := identity.Get(ctx, a.conn)
	if err != nil {
		return err
	}

	if parentID > 0 {
		parentWorkItem, err := a.Client.Get(ctx, parentID)
		if err != nil {
			return err
		}
		parent = workitem.GetReference(parentWorkItem)
		iterationPath = workitem.GetIterationPath(parentWorkItem)
	} else {
		iterationPath, err = work.GetCurrentIteration(ctx, a.conn, a.project, a.team)
		if err != nil {
			return err
		}

		parent, err = a.Client.FindCommonUserStory(ctx, iterationPath)
		if err != nil {
			return err
		}
	}

	parentRelation := workitem.Relation{
		URL:  *parent.Url,
		Type: "System.LinkTypes.Hierarchy-Reverse",
	}
	relations = append(relations, &parentRelation)

	task, err := a.Client.Create(ctx, title, description, iterationPath, int(estimate), relations, tags)
	if err != nil {
		return err
	}

	href := workitem.GetURL(task)
	fmt.Printf("%s\n", href)

	err = a.Client.Assign(ctx, task, user)
	if err != nil {
		return err
	}

	if openBrowser {
		browser.OpenURL(href)
	}

	return nil
}

func (a *API) CreateFeatureTask(ctx context.Context, title, description string, estimate int, feature *workitemtracking.WorkItem) (*workitemtracking.WorkItem, error) {
	iterationPath := workitem.GetIterationPath(feature)
	relations := []*workitem.Relation{
		{
			URL:  *feature.Url,
			Type: "System.LinkTypes.Hierarchy-Reverse",
		},
	}

	return a.Client.Create(ctx, title, description, iterationPath, int(estimate), relations, []string{})
}
