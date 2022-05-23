package pr

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"tasker/clipboard"
	"tasker/prettyprint"
	"tasker/ptr"
	"tasker/tfs/identity"

	"golang.org/x/exp/slices"

	"github.com/erikgeiser/promptkit/confirmation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/webapi"
	"github.com/spf13/viper"
)

var (
	workItemIDRegexp = regexp.MustCompile(`\d{5,6}`)

	ErrLastCommitNotFound = errors.New("last commit not found")
	ErrAborted            = errors.New("aborted")
)

type Creator struct {
	Client       *Client
	RepositoryId *string
	Project      *string
	user         *identity.Identity
}

func NewCreator(ctx context.Context, client *Client, repository, project string) (*Creator, error) {
	user, err := identity.Get(ctx, client.conn)
	if err != nil {
		return nil, err
	}

	return &Creator{
		Client:       client,
		RepositoryId: &repository,
		Project:      &project,
		user:         user,
	}, nil
}

func (c *Creator) Create(ctx context.Context, squash bool) (*git.GitPullRequest, error) {
	commit, err := c.findLastCommit(ctx)
	if err != nil {
		return nil, err
	}

	sourceBranch, targetBranch, err := c.findBranches(ctx, *commit.CommitId)
	if err != nil {
		return nil, err
	}

	prArgs := createPrArgs(*c.Project, *c.RepositoryId, sourceBranch, targetBranch, *commit.Comment, getWorkItems(commit))

	prettyprint.JSONObjectColor(prArgs)

	input := confirmation.New("Continue?", confirmation.Yes)
	confirmed, err := input.RunPrompt()
	if err != nil {
		return nil, err
	}

	if !confirmed {
		return nil, ErrAborted
	}

	pr, err := c.Client.CreatePullRequest(ctx, *prArgs)
	if err != nil {
		return nil, fmt.Errorf("pr create error: %w", err)
	}

	err = copyToClipboard(pr)
	if err != nil {
		fmt.Printf("clipboard error: %v\n", err)
	}

	err = c.setAutoComplete(ctx, pr.PullRequestId, *commit.Comment, squash)
	if err != nil {
		return pr, fmt.Errorf("set autocomplete error: %w", err)
	}

	return pr, nil
}

func copyToClipboard(pr *git.GitPullRequest) error {
	url := GetPullRequestUrl(pr)
	text := fmt.Sprintf("Pull Request %d: %s", *pr.PullRequestId, *pr.Title)
	repShortName := getRepositoryShortName(*pr.Repository.Name)
	html := fmt.Sprintf("<span>:%s: </span><a href=\"%s\">Pull Request %d</a><span>: </span><span>%s</span>", repShortName, url, *pr.PullRequestId, *pr.Title)

	return clipboard.Write(html, text)
}

func getRepositoryShortName(name string) string {
	switch name {
	case "security_management_platform":
		return "smp"
	default:
		return name
	}
}

func GetPullRequestUrl(pr *git.GitPullRequest) string {
	tfsBaseAddress := viper.GetString("tfsBaseAddress")
	return fmt.Sprintf("%s/%s/_git/%s/pullrequest/%d", tfsBaseAddress, *pr.Repository.Project.Name, *pr.Repository.Name, *pr.PullRequestId)
}

func (c *Creator) setAutoComplete(ctx context.Context, pullRequestId *int, message string, squash bool) error {
	update := &git.GitPullRequest{
		AutoCompleteSetBy: &webapi.IdentityRef{
			Id: &c.user.Id,
		},
	}
	if squash {
		update.CompletionOptions = &git.GitPullRequestCompletionOptions{
			DeleteSourceBranch: ptr.FromBool(true),
			MergeStrategy:      &git.GitPullRequestMergeStrategyValues.Squash,
			MergeCommitMessage: &message,
			SquashMerge:        ptr.FromBool(true),
		}
	}

	_, err := c.Client.UpdatePullRequest(ctx, git.UpdatePullRequestArgs{
		RepositoryId:           c.RepositoryId,
		Project:                c.Project,
		GitPullRequestToUpdate: update,
		PullRequestId:          pullRequestId,
	})

	return err
}

func (c *Creator) findLastCommit(ctx context.Context) (*git.GitCommitRef, error) {
	client := c.Client

	top := 10
	result, err := client.GetCommits(ctx, git.GetCommitsArgs{
		RepositoryId: c.RepositoryId,
		Project:      c.Project,
		Top:          ptr.FromInt(top),
		Skip:         ptr.FromInt(0),
		SearchCriteria: &git.GitQueryCommitsCriteria{
			Top:              ptr.FromInt(top),
			Skip:             ptr.FromInt(0),
			Author:           &c.user.DisplayName,
			User:             &c.user.DisplayName,
			IncludePushData:  ptr.FromBool(true),
			IncludeWorkItems: ptr.FromBool(true),
		},
	})
	if err != nil {
		return nil, err
	}

	commits := filter(*result, func(commit git.GitCommitRef) bool {
		return *commit.Push.PushedBy.DisplayName == c.user.DisplayName
	})

	if len(commits) == 0 {
		return nil, ErrLastCommitNotFound
	}

	commit := commits[0]

	return &commit, nil
}

func getWorkItems(commit *git.GitCommitRef) []string {
	items := []string{}
	for _, wi := range *commit.WorkItems {
		items = append(items, *wi.Id)
	}
	return items
}

func createPrArgs(project, repository, sourceBranch, targetBranch, message string, workItems []string) *git.CreatePullRequestArgs {
	pr := &git.CreatePullRequestArgs{
		RepositoryId: &repository,
		Project:      &project,
		GitPullRequestToCreate: &git.GitPullRequest{
			SourceRefName: ptr.FromStr("refs/heads/" + sourceBranch),
			TargetRefName: ptr.FromStr("refs/heads/" + targetBranch),
			Title:         &message,
			Description:   &message,
			CompletionOptions: &git.GitPullRequestCompletionOptions{
				MergeCommitMessage: &message,
			},
		},
	}

	for _, str := range []string{sourceBranch, message} {
		for _, idStr := range workItemIDRegexp.FindAllString(str, -1) {
			if len(idStr) > 1 {
				id, err := strconv.ParseUint(idStr, 10, 32)
				if err == nil {
					appendWorkItem(pr, strconv.Itoa(int(id)))
				}
			}
		}
	}

	for _, workItemId := range workItems {
		appendWorkItem(pr, workItemId)
	}

	return pr
}

func appendWorkItem(pr *git.CreatePullRequestArgs, workItemId string) {
	var refs []webapi.ResourceRef

	if pr.GitPullRequestToCreate.WorkItemRefs != nil {
		refs = *pr.GitPullRequestToCreate.WorkItemRefs
	}

	for _, ref := range refs {
		if *ref.Id == workItemId {
			return
		}
	}

	refs = append(refs, webapi.ResourceRef{
		Id: ptr.FromStr(workItemId),
	})

	pr.GitPullRequestToCreate.WorkItemRefs = &refs
}

func (c *Creator) findBranches(ctx context.Context, commitId string) (source, target string, err error) {
	result, err := c.Client.GetBranches(ctx, git.GetBranchesArgs{
		RepositoryId: c.RepositoryId,
		Project:      c.Project,
		BaseVersionDescriptor: &git.GitVersionDescriptor{
			Version:        &commitId,
			VersionOptions: &git.GitVersionOptionsValues.None,
			VersionType:    &git.GitVersionTypeValues.Commit,
		},
	})

	if err != nil {
		return "", "", nil
	}

	branches := *result
	if len(branches) < 2 {
		return "", "", errors.New("unable to detect source and target branches")
	}

	slices.SortFunc(branches, func(a, b git.GitBranchStats) bool {
		return *a.BehindCount < *b.BehindCount
	})

	sourceCandidates := filter(branches, func(br git.GitBranchStats) bool {
		return strings.HasPrefix(*br.Name, "pr/")
	})
	if len(sourceCandidates) < 1 {
		return "", "", errors.New("unable to detect source branch")
	}
	sourceBranch := sourceCandidates[0].Name

	targetCandidates := filter(branches, func(br git.GitBranchStats) bool {
		return !strings.HasPrefix(*br.Name, "pr/")
	})
	if len(targetCandidates) < 1 {
		return "", "", errors.New("unable to detect target branch")
	}
	targetBranch := targetCandidates[0].Name

	return *sourceBranch, *targetBranch, nil
}

func filter[T any](items []T, fn func(item T) bool) []T {
	filteredItems := []T{}
	for _, value := range items {
		if fn(value) {
			filteredItems = append(filteredItems, value)
		}
	}
	return filteredItems
}
