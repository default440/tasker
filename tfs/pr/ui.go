package pr

import (
	"errors"
	"regexp"
	"strings"

	"github.com/erikgeiser/promptkit/selection"
	"github.com/erikgeiser/promptkit/textinput"
)

func requestUserSelectionString(prompt string, choices []string) (string, error) {
	sp := selection.New(prompt, selection.Choices(choices))
	sp.PageSize = 5
	choice, err := sp.RunPrompt()

	if err != nil {
		return "", err
	}

	return choice.String, nil
}

func requestUserSelection[T any](prompt string, values []T, nameSelector func(value T) string) (T, error) {
	choices := make([]*selection.Choice, 0, len(values))
	for i := 0; i < len(values); i++ {
		choices = append(choices, &selection.Choice{
			Index:  i,
			String: nameSelector(values[i]),
			Value:  values[i],
		})
	}

	sp := selection.New(prompt, choices)
	sp.PageSize = 5
	choice, err := sp.RunPrompt()

	if err != nil {
		var result T
		return result, err
	}

	return choice.Value.(T), nil
}

func requestUserTextInput(prompt string, defaultValue string) (string, error) {
	input := textinput.New(prompt)
	input.InitialValue = defaultValue

	value, err := input.RunPrompt()
	if err != nil {
		return "", err
	}

	return value, nil
}

func confirmWorkItems(prompt string, values []string) ([]string, error) {
	input := textinput.New(prompt)
	input.InitialValue = strings.Join(values, ", ")
	input.Validate = func(input string) error {
		if m, err := regexp.MatchString(`^(\s*\d{4,5}\s*[,]?\s*)*$`, input); m && err == nil {
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
