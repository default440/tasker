package pr

import (
	"errors"
	"regexp"
	"strings"

	"github.com/erikgeiser/promptkit/confirmation"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/erikgeiser/promptkit/textinput"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
)

type promptkitUI struct{}

func (ui *promptkitUI) RequestUserSelectionBranch(prompt string, values []git.GitBranchStats, nameSelector func(value git.GitBranchStats) string) (git.GitBranchStats, error) {
	return promptkitRequestUserSelection(prompt, values, nameSelector)
}

func (ui *promptkitUI) RequestUserSelectionString(prompt string, choices []string) (string, error) {
	return promptkitRequestUserSelection(prompt, choices, func(value string) string { return value })
}

func promptkitRequestUserSelection[T any](prompt string, values []T, nameSelector func(value T) string) (T, error) {
	choices := make([]*selection.Choice, 0, len(values))
	for i := 0; i < len(values); i++ {
		choices = append(choices, &selection.Choice{
			Index:  i,
			String: nameSelector(values[i]),
			Value:  values[i],
		})
	}

	sp := selection.New(prompt+":", choices)
	sp.PageSize = 5
	choice, err := sp.RunPrompt()

	if err != nil {
		var result T
		return result, err
	}

	return choice.Value.(T), nil
}

func (ui *promptkitUI) RequestUserTextInput(prompt string, defaultValue string) (string, error) {
	input := textinput.New(prompt + ":")
	input.InitialValue = defaultValue

	value, err := input.RunPrompt()
	if err != nil {
		return "", err
	}

	return value, nil
}

func (ui *promptkitUI) ConfirmWorkItems(prompt string, values []string) ([]string, error) {
	input := textinput.New(prompt + ":")
	input.InitialValue = strings.Join(values, ", ")
	input.Validate = func(input string) error {
		if m, err := regexp.MatchString(workItemsRegexp, input); m && err == nil {
			return nil
		}
		return errors.New("invalid input")
	}

	value, err := input.RunPrompt()
	if err != nil {
		return nil, err
	}

	return parseWorkItemIDs([]string{value}), nil
}

func (ui *promptkitUI) Confirmation(prompt string, defaultValue bool) (bool, error) {
	defaultAnswer := confirmation.No
	if defaultValue {
		defaultAnswer = confirmation.Yes
	}
	return confirmation.New(prompt, defaultAnswer).RunPrompt()
}
