package tfs

import (
	"context"
	"errors"
	"tasker/tfs/connection"
	"tasker/tfs/identity"
	"tasker/tfs/work"
	"tasker/tfs/workitem"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	azurework "github.com/microsoft/azure-devops-go-api/azuredevops/v6/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/workitemtracking"
	"github.com/spf13/viper"
)

var (
	ErrFailedToAssign = errors.New("failed to assign task")
)

type API struct {
	WiClient *workitem.Client
	Conn     *azuredevops.Connection
	Project  string
	Team     string
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
		WiClient: client,
		Conn:     conn,
		Project:  project,
		Team:     team,
	}, nil
}

func (a *API) GetCurrentIteration(ctx context.Context) (*azurework.TeamSettingsIteration, error) {
	return work.GetCurrentIteration(ctx, a.Conn, a.Project, a.Team)
}

func (a *API) CreateWorkItem(ctx context.Context, workitemType, title, description string, estimate float32, parentID int, relations []*workitem.Relation, tags []string, parentNamePattern string, assign, currentIter bool) (*workitemtracking.WorkItem, error) {
	var err error
	var parent *workitemtracking.WorkItem
	var user string

	if assign {
		userIdentity, err := identity.Get(ctx, a.Conn)
		if err != nil {
			return nil, err
		}
		user = userIdentity.DisplayName
	}

	if parentID > 0 {
		parent, err = a.WiClient.Get(ctx, parentID)
	} else {
		parent, err = a.findCurrentRequirementByPattern(ctx, parentNamePattern)
	}
	if err != nil {
		return nil, err
	}

	parentRelation := workitem.Relation{
		URL:  *parent.Url,
		Type: "System.LinkTypes.Hierarchy-Reverse",
	}
	relations = append(relations, &parentRelation)

	var iterationPath string
	if currentIter {
		currentIterationPath, err := a.GetCurrentIteration(ctx)
		if err != nil {
			return nil, err
		}
		iterationPath = *currentIterationPath.Name
	} else {
		iterationPath = workitem.GetIterationPath(parent)
	}

	areaPath := workitem.GetAreaPath(parent)

	if description == "" {
		description = title
	}

	var workitem *workitemtracking.WorkItem
	if workitemType == "Requirement" {
		workitem, err = a.WiClient.CreateRequirement(ctx, "Development", title, description, areaPath, iterationPath, estimate, 1, relations, tags)
	} else {
		workitem, err = a.WiClient.CreateTask(ctx, title, description, areaPath, iterationPath, estimate, relations, tags, "", "", "")
	}
	if err != nil {
		return nil, err
	}

	if assign {
		err = a.WiClient.Assign(ctx, workitem, user)
		if err != nil {
			return workitem, err
		}
	}

	return workitem, nil
}

func (a *API) findActiveRequirementByPattern(ctx context.Context, namePattern string) (*workitemtracking.WorkItem, error) {
	requirement, err := a.WiClient.FindRequirement(ctx, namePattern, "", "Active")
	if err != nil {
		return nil, err
	}

	if requirement != nil {
		return requirement, nil
	}

	return nil, errors.New("active requirement with name contains '" + namePattern + "' not found")
}

func (a *API) findCurrentRequirementByPattern(ctx context.Context, namePattern string) (*workitemtracking.WorkItem, error) {
	iterations, err := work.GetIterations(ctx, a.Conn, a.Project, a.Team)
	if err != nil {
		return nil, err
	}

	for i := len(*iterations) - 1; i >= 0; i-- {
		iteration := (*iterations)[i]
		if *iteration.Attributes.TimeFrame == "current" {
			requirement, err := a.WiClient.FindRequirement(ctx, namePattern, *iteration.Path, "")
			if err != nil {
				return nil, err
			}
			if requirement != nil {
				return requirement, nil
			}
		}
	}

	return nil, errors.New("active requirement with name contains '" + namePattern + "' not found in current and previous sprints")
}

func (a *API) CreateChildTask(ctx context.Context, title, description string, estimate float32, parent *workitemtracking.WorkItem, tags []string, assignedTo string, startDate string, finishDate string) (*workitemtracking.WorkItem, error) {
	iterationPath := workitem.GetIterationPath(parent)
	areaPath := workitem.GetAreaPath(parent)
	relations := []*workitem.Relation{
		{
			URL:  *parent.Url,
			Type: "System.LinkTypes.Hierarchy-Reverse",
		},
	}

	return a.WiClient.CreateTask(ctx, title, description, areaPath, iterationPath, estimate, relations, tags, assignedTo, startDate, finishDate)
}

func (a *API) CreateChildRequirement(ctx context.Context, requirementType, title, description string, estimate, priority float32, parent *workitemtracking.WorkItem, tags []string) (*workitemtracking.WorkItem, error) {
	// iterationPath := workitem.GetIterationPath(parent)
	areaPath := workitem.GetAreaPath(parent)
	iterationPath := areaPath
	relations := []*workitem.Relation{
		{
			URL:  *parent.Url,
			Type: "System.LinkTypes.Hierarchy-Reverse",
		},
	}

	return a.WiClient.CreateRequirement(ctx, requirementType, title, description, areaPath, iterationPath, estimate, priority, relations, tags)
}
