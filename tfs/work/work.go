package work

import (
	"context"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/work"
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

func GetCurrentIteration(iterations *[]work.TeamSettingsIteration) *work.TeamSettingsIteration {
	for i := 0; i < len(*iterations); i++ {
		if *(*iterations)[i].Attributes.TimeFrame == "current" {
			return &(*iterations)[i]
		}
	}
	return nil
}

func GetPreviousIteration(iterations *[]work.TeamSettingsIteration) *work.TeamSettingsIteration {
	for i := 1; i < len(*iterations); i++ {
		if *(*iterations)[i].Attributes.TimeFrame == "current" {
			return &(*iterations)[i-1]
		}
	}
	return nil
}
