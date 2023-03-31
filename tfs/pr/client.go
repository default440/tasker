package pr

import (
	"context"
	"errors"
	"sort"
	"sync"
	"tasker/ptr"

	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
)

var (
	ErrRepositoryNotFound = errors.New("repository not found")

	priorityReps = []string{
		"security_management_platform",
		"silso",
		"smp_contracts",
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

	rep, err := ui.RequestUserSelectionString("Repository", repNames)
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
	wg, ctx := errgroup.WithContext(ctx)
	var m sync.Mutex
	for _, rep := range reps {
		repository := rep
		wg.Go(func() error {
			suggestions, err := c.GetSuggestions(ctx, git.GetSuggestionsArgs{
				RepositoryId: ptr.FromStr(repository),
				Project:      &c.project,
			})

			if err != nil {
				return nil
			}

			if len(*suggestions) > 0 {
				m.Lock()
				suggested = append(suggested, repository)
				m.Unlock()
			}

			return nil
		})
	}
	err := wg.Wait()
	if err != nil {
		return nil, err
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
