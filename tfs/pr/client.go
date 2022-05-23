package pr

import (
	"context"
	"errors"
	"sort"
	"tasker/ptr"

	"golang.org/x/exp/slices"

	"github.com/erikgeiser/promptkit/selection"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
)

var (
	ErrRepositoryNotFound = errors.New("repository not found")

	priorityReps = []string{
		"security_management_platform",
		"silso",
	}
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

func (c *Client) RequestRepository(ctx context.Context) (string, error) {
	reps, err := c.GetRepositories(ctx, git.GetRepositoriesArgs{
		Project: &c.project,
	})
	if err != nil {
		return "", err
	}

	repNames := getRepositoryNames(*reps)
	repNames, err = prioritizeReps(ctx, c, repNames)
	if err != nil {
		return "", err
	}

	rep, err := requestUserSelection(repNames)
	if err != nil {
		return "", err
	}
	if rep != "" {
		return rep, nil
	}

	return "", ErrRepositoryNotFound
}

func prioritizeReps(ctx context.Context, c *Client, reps []string) ([]string, error) {
	var suggested []string
	for _, rep := range reps {
		suggestions, err := c.GetSuggestions(ctx, git.GetSuggestionsArgs{
			RepositoryId: ptr.FromStr(rep),
			Project:      &c.project,
		})
		if err != nil {
			return nil, err
		}

		if len(*suggestions) > 0 {
			suggested = append(suggested, rep)
			break
		}
	}

	sort.Strings(suggested)

	result := make([]string, len(suggested))
	copy(result, suggested)

	result = appendUnique(result, priorityReps)
	result = appendUnique(result, reps)

	return result, nil
}

func appendUnique[E comparable](s []E, v []E) []E {
	for _, x := range v {
		if !slices.Contains(s, x) {
			s = append(s, x)
		}
	}
	return s
}

func getRepositoryNames(reps []git.GitRepository) []string {
	repNames := make([]string, 0, len(reps))
	for _, r := range reps {
		repNames = append(repNames, *r.Name)
	}

	sort.Strings(repNames)

	return repNames
}

func requestUserSelection(choices []string) (string, error) {
	sp := selection.New("Repository", selection.Choices(choices))
	sp.PageSize = 5

	choice, err := sp.RunPrompt()

	if err != nil {
		return "", err
	}

	return choice.String, nil
}
