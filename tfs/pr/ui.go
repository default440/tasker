package pr

import "github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"

const (
	workItemsRegexp      = `^(\s*\d{5,6}\s*[,]?\s*)*$`
	workItemsInputRegexp = `^(\s*\d{5,6}\s*,\s*)*(\s*\d{1,6}\s*[,]?\s*)?$`
)

var (
	ui UI = &promptkitUI{}
	// ui UI = &promptuiUI{}
	// ui UI = &ptermUI{}
	// ui UI = &surveyUI{}
	// ui UI = &tviewUI{}
)

type UserSelections struct {
	SourceBranch    *git.GitBranchStats `validate:"required"`
	TargetBranch    *git.GitBranchStats `validate:"required"`
	Repository      string              `validate:"required"`
	WorkItems       []string            `validate:"required"`
	MergeMessage    string              `validate:"required"`
	Squash          bool
	WithWorkItemIDs bool
}

type UI interface {
	RequestUserSelectionBranch(prompt string, values []git.GitBranchStats, nameSelector func(value git.GitBranchStats) string) (git.GitBranchStats, error)
	RequestUserSelectionString(prompt string, choices []string) (string, error)
	RequestUserTextInput(prompt string, defaultValue string) (string, error)
	ConfirmWorkItems(prompt string, values []string) ([]string, error)
	Confirmation(prompt string, defaultValue bool) (bool, error)
}

type InteractiveUI interface {
	SetRepositories(repositories []string)
	SetSourceBranches(branches []git.GitBranchStats)
	SetTargetBranches(branches []git.GitBranchStats)
	SetWorkItems(workItemIDs []string)
	SetMergeMessage(message string)

	SetRepositoryChangeHandler(handler func(repository string))
	SetTargetBranchChangeHandler(handler func(targetBranch git.GitBranchStats))
	SetSourceBranchChangeHandler(handler func(sourceBranch git.GitBranchStats))
	SetCreateHandler(handler func(selections UserSelections))
	SetCancelHandler(handler func())
	SetErrHandler(handler func(error))
}
