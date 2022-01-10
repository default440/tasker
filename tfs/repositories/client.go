package repositories

import (
	"context"
	"log"
	"path/filepath"
	"tasker/ptr"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
)

type Client struct {
	client git.Client
}

type RepositoryBranches struct {
	Name     string
	Branches []string
}

func NewClient(ctx context.Context, conn *azuredevops.Connection) (*Client, error) {
	client, err := git.NewClient(ctx, conn)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: client,
	}, nil
}

func (c *Client) GetBranches(ctx context.Context, project, filter string) ([]RepositoryBranches, error) {
	gitReps, err := c.client.GetRepositories(ctx, git.GetRepositoriesArgs{
		Project: &project,
	})
	if err != nil {
		return nil, err
	}

	var results []RepositoryBranches

	for _, rep := range *gitReps {
		branches, err := c.client.GetBranches(ctx, git.GetBranchesArgs{
			Project:      &project,
			RepositoryId: ptr.FromStr(rep.Id.String()),
		})

		if err != nil {
			log.Println(err.Error())
		} else {
			result := RepositoryBranches{
				Name: *rep.Name,
			}

			for _, branch := range *branches {
				if matched, _ := filepath.Match(filter, *branch.Name); matched {
					result.Branches = append(result.Branches, *branch.Name)
				}
			}

			if len(result.Branches) > 0 {
				results = append(results, result)
			}
		}
	}

	return results, nil
}
