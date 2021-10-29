package work

import (
	"context"
	"errors"
	"tasker/ptr"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/work"
)

func GetCurrentIteration(ctx context.Context, conn *azuredevops.Connection, project, team string) (string, error) {
	client, err := work.NewClient(ctx, conn)
	if err != nil {
		return "", err
	}

	iterations, err := client.GetTeamIterations(ctx, work.GetTeamIterationsArgs{
		Timeframe: ptr.FromStr("current"),
		Project:   &project,
		Team:      &team,
	})
	if err != nil {
		return "", err
	}

	if len(*iterations) != 1 {
		return "", errors.New("current iteration not exists")
	}

	return *(*iterations)[0].Path, nil
}
