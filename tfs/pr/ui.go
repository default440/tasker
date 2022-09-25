package pr

import "github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"

const (
	workItemsRegexp = `^(\s*\d{5}\s*[,]?\s*)*$`
)

var (
	ui UI = &promptkitUI{}
)

type UI interface {
	RequestUserSelectionBranch(prompt string, values []git.GitBranchStats, nameSelector func(value git.GitBranchStats) string) (git.GitBranchStats, error)
	RequestUserSelectionString(prompt string, choices []string) (string, error)
	RequestUserTextInput(prompt string, defaultValue string) (string, error)
	ConfirmWorkItems(prompt string, values []string) ([]string, error)
	Confirmation(prompt string, defaultValue bool) (bool, error)
}
