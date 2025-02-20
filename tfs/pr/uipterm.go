package pr

import (
	"errors"
	"regexp"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	"github.com/pterm/pterm"
)

type ptermUI struct{}

func (ui *ptermUI) RequestUserSelectionBranch(prompt string, values []git.GitBranchStats, nameSelector func(value git.GitBranchStats) string) (git.GitBranchStats, error) {
	return ptermRequestUserSelection(prompt, values, nameSelector)
}

func (ui *ptermUI) RequestUserSelectionString(prompt string, choices []string) (string, error) {
	return ptermRequestUserSelection(prompt, choices, func(value string) string { return value })
}

func ptermRequestUserSelection[T any](prompt string, values []T, nameSelector func(value T) string) (T, error) {
	options := make([]string, 0, len(values))

	for i := 0; i < len(values); i++ {
		options = append(options, nameSelector(values[i]))
	}

	option, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		Show(prompt)

	if err == nil {
		for i := 0; i < len(values); i++ {
			if option == nameSelector(values[i]) {
				return values[i], nil
			}
		}
	}

	var result T
	if err != nil {
		return result, err
	}

	return result, errors.New("unable to determine selected value")
}

func (ui *ptermUI) RequestUserTextInput(prompt string, defaultValue string) (string, error) {
	value, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText(defaultValue).
		WithMultiLine(false).
		Show(prompt)

	if err != nil {
		return "", err
	}

	return value, nil
}

func (ui *ptermUI) ConfirmWorkItems(prompt string, values []string) ([]string, error) {
	var err error
	input := strings.Join(values, ", ")

start:
	input, err = ui.RequestUserTextInput(prompt, input)
	if err != nil {
		return nil, err
	}

	if m, err := regexp.MatchString(workItemsRegexp, input); m && err == nil {
		return parseWorkItemIDs([]string{input}), nil
	}

	pterm.Error.Printfln("invalid input: %s", input)

	goto start
}

func (ui *ptermUI) Confirmation(prompt string, defaultValue bool) (bool, error) {
	yes := "Yes"
	no := "No"
	choices := []string{
		yes,
		no,
	}

	value, err := ui.RequestUserSelectionString(prompt, choices)
	return value == yes, err
}
