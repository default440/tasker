package repositories

import (
	"context"
	"strings"
	"tasker/ptr"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
)

type Client struct {
	client git.Client
}

type Repository struct {
	Name     string
	Branches []*Branch
}

type Branch struct {
	git.GitRef
	RepositoryID *uuid.UUID
}

func (b *Branch) DisplayName() string {
	return strings.TrimPrefix(*b.Name, "refs/heads/")
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

func (c *Client) GetBranches(ctx context.Context, project, filter string) ([]Repository, error) {
	gitReps, err := c.client.GetRepositories(ctx, git.GetRepositoriesArgs{
		Project: &project,
	})
	if err != nil {
		return nil, err
	}

	var results []Repository

	for _, rep := range *gitReps {
		branches, err := c.client.GetRefs(ctx, git.GetRefsArgs{
			Project:      &project,
			RepositoryId: ptr.FromStr(rep.Id.String()),
			Filter:       ptr.FromStr("heads/" + filter),
		})

		if err == nil {
			result := Repository{
				Name: *rep.Name,
			}

			for _, branch := range branches.Value {
				result.Branches = append(result.Branches, &Branch{
					branch,
					rep.Id,
				})
			}

			if len(result.Branches) > 0 {
				results = append(results, result)
			}
		}
	}

	return results, nil
}

func (c *Client) DeleteBranches(ctx context.Context, project, repository string, branches []*Branch) error {

	nilUID := ptr.FromStr("0000000000000000000000000000000000000000")

	var refUpdates []git.GitRefUpdate

	for _, branch := range branches {
		refUpdates = append(refUpdates, git.GitRefUpdate{
			Name:         branch.Name,
			OldObjectId:  branch.ObjectId,
			NewObjectId:  nilUID,
			RepositoryId: branch.RepositoryID,
		})
	}

	_, err := c.client.UpdateRefs(ctx, git.UpdateRefsArgs{
		Project:      ptr.FromStr(project),
		RepositoryId: ptr.FromStr(repository),
		RefUpdates:   &refUpdates,
	})

	return err
}
