package pr

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"tasker/clipboard"
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
	RepositoryID *string
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
		RepositoryID: &repository,
		Project:      &project,
		user:         user,
	}, nil
}

func (c *Creator) Create(ctx context.Context) (*git.GitPullRequest, error) {
	commit, err := c.findLastCommit(ctx)
	if err != nil {
		return nil, err
	}

	sourceBranch, targetBranch, err := c.findBranches(ctx, *commit.CommitId)
	if err != nil {
		return nil, err
	}

	message, err := requestUserTextInput("Merge commit message:", *commit.Comment)
	if err != nil {
		return nil, err
	}

	workItems := getWorkItems(commit)
	workItems, err = confirmWorkItems("Work items:", workItems)
	if err != nil {
		return nil, err
	}

	prArgs := createPrArgs(
		*c.Project,
		*c.RepositoryID,
		sourceBranch,
		targetBranch,
		message,
		workItems,
	)

	// prettyprint.JSONObjectColor(prArgs)

	squash, err := confirmation.New("Squash pr?", confirmation.Yes).RunPrompt()
	if err != nil {
		return nil, err
	}

	confirmed, err := confirmation.New("Continue?", confirmation.Yes).RunPrompt()
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

	err = c.setAutoComplete(ctx, pr.PullRequestId, message, squash)
	if err != nil {
		return pr, fmt.Errorf("set autocomplete error: %w", err)
	}

	return pr, nil
}

func copyToClipboard(pr *git.GitPullRequest) error {
	url := GetPullRequestURL(pr)
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

func GetPullRequestURL(pr *git.GitPullRequest) string {
	tfsBaseAddress := viper.GetString("tfsBaseAddress")
	return fmt.Sprintf("%s/%s/_git/%s/pullrequest/%d", tfsBaseAddress, *pr.Repository.Project.Name, *pr.Repository.Name, *pr.PullRequestId)
}

func (c *Creator) setAutoComplete(ctx context.Context, pullRequestID *int, message string, squash bool) error {
	update := &git.GitPullRequest{
		AutoCompleteSetBy: &webapi.IdentityRef{
			Id: &c.user.Id,
		},
		CompletionOptions: &git.GitPullRequestCompletionOptions{
			MergeCommitMessage: &message,
		},
	}
	if squash {
		update.CompletionOptions = &git.GitPullRequestCompletionOptions{
			DeleteSourceBranch: ptr.FromBool(true),
			MergeStrategy:      &git.GitPullRequestMergeStrategyValues.Squash,
			SquashMerge:        ptr.FromBool(true),
		}
	} else {
		update.CompletionOptions = &git.GitPullRequestCompletionOptions{
			DeleteSourceBranch: ptr.FromBool(false),
			MergeStrategy:      &git.GitPullRequestMergeStrategyValues.NoFastForward,
		}
	}

	_, err := c.Client.UpdatePullRequest(ctx, git.UpdatePullRequestArgs{
		RepositoryId:           c.RepositoryID,
		Project:                c.Project,
		GitPullRequestToUpdate: update,
		PullRequestId:          pullRequestID,
	})

	return err
}

func (c *Creator) findLastCommit(ctx context.Context) (*git.GitCommitRef, error) {
	client := c.Client

	top := 10
	result, err := client.GetCommits(ctx, git.GetCommitsArgs{
		RepositoryId: c.RepositoryID,
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

	for _, workItemID := range parseWorkItemIDs([]string{sourceBranch, message}) {
		appendWorkItem(pr, workItemID)
	}

	for _, workItemID := range workItems {
		appendWorkItem(pr, workItemID)
	}

	return pr
}

func parseWorkItemIDs(inputs []string) []string {
	var workItems []string
	for _, str := range inputs {
		for _, idStr := range workItemIDRegexp.FindAllString(str, -1) {
			if len(idStr) > 1 {
				id, err := strconv.ParseUint(idStr, 10, 32)
				if err == nil {
					workItems = append(workItems, strconv.Itoa(int(id)))
				}
			}
		}
	}
	return workItems
}

func appendWorkItem(pr *git.CreatePullRequestArgs, workItemID string) {
	var refs []webapi.ResourceRef

	if pr.GitPullRequestToCreate.WorkItemRefs != nil {
		refs = *pr.GitPullRequestToCreate.WorkItemRefs
	}

	for _, ref := range refs {
		if *ref.Id == workItemID {
			return
		}
	}

	refs = append(refs, webapi.ResourceRef{
		Id: ptr.FromStr(workItemID),
	})

	pr.GitPullRequestToCreate.WorkItemRefs = &refs
}

func (c *Creator) findBranches(ctx context.Context, commitID string) (source, target string, err error) {
	result, err := c.Client.GetBranches(ctx, git.GetBranchesArgs{
		RepositoryId: c.RepositoryID,
		Project:      c.Project,
		BaseVersionDescriptor: &git.GitVersionDescriptor{
			Version:        &commitID,
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

	branchNames := project(branches, func(b git.GitBranchStats) string {
		return *b.Name
	})

	sourceCandidates := filter(branchNames, func(branch string) bool {
		return strings.HasPrefix(branch, "pr/")
	})
	if len(sourceCandidates) < 1 {
		return "", "", errors.New("unable to detect source branch")
	}
	sourceBranch, err := requestUserSelection("Source branch:", sourceCandidates)
	if err != nil {
		return "", "", err
	}

	targetCandidates := filter(branchNames, func(branch string) bool {
		return !strings.HasPrefix(branch, "pr/")
	})
	if len(targetCandidates) < 1 {
		return "", "", errors.New("unable to detect target branch")
	}
	targetBranch, err := requestUserSelection("Target branch:", targetCandidates)
	if err != nil {
		return "", "", err
	}

	return sourceBranch, targetBranch, nil
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

func project[T any, TResult any](items []T, selector func(item T) TResult) []TResult {
	result := []TResult{}
	for _, item := range items {
		result = append(result, selector(item))
	}
	return result
}
