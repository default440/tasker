package pr

import (
	"context"
	"errors"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
)

var (
	ErrRepositoryNotFound = errors.New("repository not found")
)

type Client struct {
	git.Client
	conn    *azuredevops.Connection
	project string
}

func NewClient(ctx context.Context, conn *azuredevops.Connection, project string) (*Client, error) {
	client, err := git.NewClient(ctx, conn)
	if err != nil {
		return nil, err
	}

	return &Client{
		client,
		conn,
		project,
	}, nil
}

func (c *Client) NewCreator(ctx context.Context, repository string) (*Creator, error) {
	return NewCreator(ctx, c, repository, c.project)
}

func (c *Client) FindRepository(ctx context.Context) (string, error) {
	reps, err := c.GetRepositories(ctx, git.GetRepositoriesArgs{
		Project: &c.project,
	})
	if err != nil {
		return "", err
	}

	for _, rep := range *reps {
		suggestions, err := c.GetSuggestions(ctx, git.GetSuggestionsArgs{
			RepositoryId: rep.Name,
			Project:      &c.project,
		})
		if err != nil {
			return "", err
		}

		if len(*suggestions) > 0 {
			return *rep.Name, nil
		}
	}

	return "", ErrRepositoryNotFound
}
