package work

import (
	"context"
	"errors"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/work"
)

func GetIterations(ctx context.Context, conn *azuredevops.Connection, project, team string) (*[]work.TeamSettingsIteration, error) {
	client, err := work.NewClient(ctx, conn)
	if err != nil {
		return nil, err
	}

	return client.GetTeamIterations(ctx, work.GetTeamIterationsArgs{
		Project: &project,
		Team:    &team,
	})
}

func GetCurrentIteration(ctx context.Context, conn *azuredevops.Connection, project, team string) (*work.TeamSettingsIteration, error) {
	iterations, err := GetIterations(ctx, conn, project, team)
	if err != nil {
		return nil, err
	}

	currentIter := FindCurrentIteration(iterations)
	if currentIter != nil {
		return currentIter, nil
	}

	return nil, errors.New("current iteration not found")
}

func FindCurrentIteration(iterations *[]work.TeamSettingsIteration) *work.TeamSettingsIteration {
	for i := 0; i < len(*iterations); i++ {
		if *(*iterations)[i].Attributes.TimeFrame == "current" {
			return &(*iterations)[i]
		}
	}
	return nil
}

func FindPreviousIteration(iterations *[]work.TeamSettingsIteration) *work.TeamSettingsIteration {
	for i := 1; i < len(*iterations); i++ {
		if *(*iterations)[i].Attributes.TimeFrame == "current" {
			return &(*iterations)[i-1]
		}
	}
	return nil
}
