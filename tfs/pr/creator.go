package pr

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"tasker/clipboard"
	"tasker/ptr"
	"tasker/tfs/identity"
	"tasker/tfs/workitem"

	"golang.org/x/exp/slices"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/webapi"
	"github.com/samber/lo"
	"github.com/spf13/viper"
)

var (
	workItemIDRegexp = regexp.MustCompile(`\d{5,6}`)

	ErrAborted = errors.New("aborted")
)

type Creator struct {
	Client       *Client
	WiClient     *workitem.Client
	RepositoryID *string
	Project      *string
	user         *identity.Identity
	userCommits  []git.GitCommitRef
}

func NewCreator(ctx context.Context, client *Client, wiClient *workitem.Client, repository, project string) (*Creator, error) {
	user, err := identity.Get(ctx, client.conn)
	if err != nil {
		return nil, err
	}

	c := Creator{
		Client:       client,
		WiClient:     wiClient,
		RepositoryID: &repository,
		Project:      &project,
		user:         user,
	}

	userCommits, err := c.getUserCommits(ctx)
	if err != nil {
		return nil, err
	}
	c.userCommits = userCommits

	return &c, nil
}

func (c *Creator) Create(ctx context.Context, message, description string) (*git.GitPullRequest, error) {
	sourceBranch, targetBranch, err := c.getSourceTargetBranches(ctx)
	if err != nil {
		return nil, err
	}

	if message == "" {
		message, err = c.SuggestMergeMessage(ctx, targetBranch)
		if err != nil {
			return nil, err
		}
	}
	message, err = ui.RequestUserTextInput("Merge commit message", message)
	if err != nil {
		return nil, err
	}

	workItems := c.CollectWorkItems(sourceBranch, message)
	workItems, err = ui.ConfirmWorkItems("Work items", workItems)
	if err != nil {
		return nil, err
	}

	squash, err := ui.Confirmation("Squash pr?", true)
	if err != nil {
		return nil, err
	}

	confirmed, err := ui.Confirmation("Continue?", true)
	if err != nil {
		return nil, err
	}

	if !confirmed {
		return nil, ErrAborted
	}

	pr, err := c.CreatePullRequest(ctx, sourceBranch, targetBranch, message, description, workItems, squash)
	if err != nil {
		return pr, err
	}

	return pr, nil
}

func (c *Creator) CreatePullRequest(ctx context.Context, sourceBranch *git.GitBranchStats, targetBranch *git.GitBranchStats, message, description string, workItems []string, squash bool) (*git.GitPullRequest, error) {
	prArgs, err := c.CreatePrArgs(
		ctx,
		*c.Project,
		*c.RepositoryID,
		*sourceBranch.Name,
		*targetBranch.Name,
		message,
		description,
		workItems,
	)
	if err != nil {
		return nil, fmt.Errorf("pr args create error: %w", err)
	}

	pr, err := c.Client.CreatePullRequest(ctx, *prArgs)
	if err != nil {
		return nil, fmt.Errorf("pr create error: %w", err)
	}

	err = copyToClipboard(pr)
	if err != nil {
		fmt.Printf("clipboard error: %v\n", err)
	}

	err = c.setAutoComplete(ctx, pr.PullRequestId, *prArgs.GitPullRequestToCreate.CompletionOptions.MergeCommitMessage, squash)
	if err != nil {
		return pr, fmt.Errorf("set autocomplete error: %w", err)
	}
	return pr, nil
}

func (c *Creator) CollectWorkItems(sourceBranch *git.GitBranchStats, mergeMessage string) []string {
	var workItems []string
	if lastCommit, ok := c.getLastCommit(); ok {
		workItems = getWorkItems(lastCommit)
	}
	workItems = append(workItems, parseWorkItemIDs([]string{*sourceBranch.Name, mergeMessage})...)
	slices.Sort(workItems)
	slices.Compact(workItems)
	return workItems
}

func (c *Creator) SuggestMergeMessage(ctx context.Context, targetBranch *git.GitBranchStats) (string, error) {
	if initialPrBranchCommit, found := getInitialPrBranchCommit(c.userCommits, targetBranch); found {
		return c.GetFullCommitMessage(ctx, initialPrBranchCommit)
	}

	if lastCommit, ok := c.getLastCommit(); ok {
		return c.GetFullCommitMessage(ctx, lastCommit)
	}
	return "", nil
}

func (c *Creator) GetFullCommitMessage(ctx context.Context, commitRef *git.GitCommitRef) (string, error) {
	commit, err := c.Client.GetCommit(ctx, git.GetCommitArgs{
		RepositoryId: c.RepositoryID,
		Project:      c.Project,
		CommitId:     commitRef.CommitId,
	})

	if err != nil {
		return "", err
	}

	return *commit.Comment, nil
}

func copyToClipboard(pr *git.GitPullRequest) error {
	url := GetPullRequestURL(pr)
	text := fmt.Sprintf("Pull Request %d: %s", *pr.PullRequestId, *pr.Title)
	repShortName := getRepositoryShortName(*pr.Repository.Name)
	repSpecialMark := getRepositorySpecialMark(*pr.Repository.Name)
	if repSpecialMark != "" {
		repSpecialMark += " "
	}
	html := fmt.Sprintf("<span>:%s: %s</span><a href=\"%s\">Pull Request %d</a><span>: </span><span>%s</span>", repShortName, repSpecialMark, url, *pr.PullRequestId, *pr.Title)

	return clipboard.Write(html, text)
}

func getRepositoryShortName(name string) string {
	switch name {
	case "security_management_platform":
		return "smp"
	case "angular_ui_components":
		return "ang"
	default:
		return name
	}
}

func getRepositorySpecialMark(name string) string {
	switch name {
	case "security_management_platform":
		return "[MS]"
	case "smp_kc":
		return "[KC]"
	default:
		return ""
	}
}

func GetPullRequestURL(pr *git.GitPullRequest) string {
	tfsBaseAddress := viper.GetString("tfsBaseAddress")
	return fmt.Sprintf("%s/%s/_git/%s/pullrequest/%d", tfsBaseAddress, *pr.Repository.Project.Name, *pr.Repository.Name, *pr.PullRequestId)
}

func (c *Creator) setAutoComplete(ctx context.Context, pullRequestID *int, message string, squash bool) error {
	mergeCommitMessage := fmt.Sprintf("Merged PR %d: %s", *pullRequestID, message)
	update := &git.GitPullRequest{
		AutoCompleteSetBy: &webapi.IdentityRef{
			Id: &c.user.Id,
		},
		CompletionOptions: &git.GitPullRequestCompletionOptions{
			MergeCommitMessage: &mergeCommitMessage,
		},
	}
	if squash {
		update.CompletionOptions.DeleteSourceBranch = ptr.FromBool(true)
		update.CompletionOptions.MergeStrategy = &git.GitPullRequestMergeStrategyValues.Squash
		update.CompletionOptions.SquashMerge = ptr.FromBool(true)
	} else {
		update.CompletionOptions.DeleteSourceBranch = ptr.FromBool(false)
		update.CompletionOptions.MergeStrategy = &git.GitPullRequestMergeStrategyValues.NoFastForward
	}

	_, err := c.Client.UpdatePullRequest(ctx, git.UpdatePullRequestArgs{
		RepositoryId:           c.RepositoryID,
		Project:                c.Project,
		GitPullRequestToUpdate: update,
		PullRequestId:          pullRequestID,
	})

	return err
}

func (c *Creator) getUserCommits(ctx context.Context) ([]git.GitCommitRef, error) {
	client := c.Client

	top := 30
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

	return commits, nil
}

func (c *Creator) getLastCommit() (*git.GitCommitRef, bool) {
	if len(c.userCommits) == 0 {
		return nil, false
	}

	return &(c.userCommits[0]), true
}

func getInitialPrBranchCommit(commits []git.GitCommitRef, targetBranch *git.GitBranchStats) (*git.GitCommitRef, bool) {
	if len(commits) == 0 {
		return nil, false
	}

	if len(commits) < *targetBranch.BehindCount {
		return nil, false
	}

	if *targetBranch.BehindCount == 0 {
		return &commits[0], true
	}

	return &commits[(*targetBranch.BehindCount)-1], true
}

func getWorkItems(commit *git.GitCommitRef) []string {
	items := []string{}
	for _, wi := range *commit.WorkItems {
		items = append(items, *wi.Id)
	}
	return items
}

func (c *Creator) CreatePrArgs(ctx context.Context, project, repository, sourceBranch, targetBranch, message, description string, workItems []string) (*git.CreatePullRequestArgs, error) {
	mergeCommitMessage := message

	// if len(description) > 0 {
	// 	mergeCommitMessage += "\n\n" + description
	// }

	if len(workItems) > 0 {
		mergeCommitMessage += "\n\n"

		for _, workItem := range workItems {
			id, err := strconv.Atoi(workItem)
			if err != nil {
				return nil, fmt.Errorf("parse workitem id error: %w", err)
			}

			wi, err := c.WiClient.Get(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("get workitem error: %w", err)
			}

			mergeCommitMessage += fmt.Sprintf("#%s %s\n", workItem, workitem.GetTitle(wi))
		}

		mergeCommitMessage += "\nRelated work items: " + strings.Join(lo.Map(workItems, func(x string, _ int) string { return "#" + x }), ", ")
	}

	pr := &git.CreatePullRequestArgs{
		RepositoryId: &repository,
		Project:      &project,
		GitPullRequestToCreate: &git.GitPullRequest{
			SourceRefName: ptr.FromStr("refs/heads/" + sourceBranch),
			TargetRefName: ptr.FromStr("refs/heads/" + targetBranch),
			Title:         &message,
			Description:   &description,
			CompletionOptions: &git.GitPullRequestCompletionOptions{
				MergeCommitMessage: &mergeCommitMessage,
			},
		},
	}

	for _, workItemID := range workItems {
		appendWorkItem(pr, workItemID)
	}

	return pr, nil
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

func (c *Creator) GetBranchCandidates(ctx context.Context) (source, target []git.GitBranchStats, err error) {
	var version *string

	if lastCommit, ok := c.getLastCommit(); ok {
		version = lastCommit.CommitId
	}

	result, err := c.Client.GetBranches(ctx, git.GetBranchesArgs{
		RepositoryId: c.RepositoryID,
		Project:      c.Project,
		BaseVersionDescriptor: &git.GitVersionDescriptor{
			Version:        version,
			VersionOptions: &git.GitVersionOptionsValues.None,
			VersionType:    &git.GitVersionTypeValues.Commit,
		},
	})

	if err != nil {
		return nil, nil, nil
	}

	branches := *result

	sort.Slice(branches, func(a, b int) bool {
		return *branches[a].BehindCount < *branches[b].BehindCount
	})

	sourceCandidates := filter(branches, func(branch git.GitBranchStats) bool {
		return *branch.Commit.Committer.Name == c.user.DisplayName
	})

	targetCandidates := filter(branches, func(branch git.GitBranchStats) bool {
		return !strings.HasPrefix(*branch.Name, "pr/")
	})

	return sourceCandidates, targetCandidates, nil
}

func (c *Creator) getSourceTargetBranches(ctx context.Context) (source, target *git.GitBranchStats, err error) {
	sourceCandidates, targetCandidates, err := c.GetBranchCandidates(ctx)
	if err != nil {
		return nil, nil, err
	}

	if len(sourceCandidates) < 1 {
		return nil, nil, errors.New("unable to detect source branch")
	}

	if len(targetCandidates) < 1 {
		return nil, nil, errors.New("unable to detect target branch")
	}

	branchNameSelector := func(branch git.GitBranchStats) string {
		return *branch.Name
	}

	sourceBranch, err := ui.RequestUserSelectionBranch("Source branch", sourceCandidates, branchNameSelector)
	if err != nil {
		return nil, nil, err
	}

	targetBranch, err := ui.RequestUserSelectionBranch("Target branch", targetCandidates, branchNameSelector)
	if err != nil {
		return nil, nil, err
	}

	return &sourceBranch, &targetBranch, nil
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
