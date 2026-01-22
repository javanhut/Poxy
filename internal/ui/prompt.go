package ui

import (
	"fmt"
	"strings"

	"poxy/pkg/manager"

	"github.com/manifoldco/promptui"
)

// Confirm prompts the user for yes/no confirmation.
func Confirm(prompt string, defaultYes bool) (bool, error) {
	label := prompt
	if defaultYes {
		label += " [Y/n]"
	} else {
		label += " [y/N]"
	}

	p := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
		Default:   "",
	}

	if defaultYes {
		p.Default = "y"
	}

	result, err := p.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return defaultYes, nil // Return default on error
	}

	result = strings.ToLower(strings.TrimSpace(result))
	if result == "" {
		return defaultYes, nil
	}

	return result == "y" || result == "yes", nil
}

// SelectPackage prompts the user to select a package from a list.
func SelectPackage(packages []manager.Package, prompt string) (*manager.Package, error) {
	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages to select from")
	}

	if len(packages) == 1 {
		return &packages[0], nil
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Name | cyan }} {{ .Version | green }} [{{ .Source | magenta }}]",
		Inactive: "  {{ .Name }} {{ .Version | faint }} [{{ .Source | faint }}]",
		Selected: "✓ {{ .Name | cyan }} {{ .Version | green }} [{{ .Source | magenta }}]",
		Details: `
--------- Package ----------
{{ "Name:" | faint }}	{{ .Name }}
{{ "Version:" | faint }}	{{ .Version }}
{{ "Source:" | faint }}	{{ .Source }}
{{ "Description:" | faint }}	{{ .Description }}`,
	}

	searcher := func(input string, index int) bool {
		pkg := packages[index]
		name := strings.ToLower(pkg.Name)
		input = strings.ToLower(input)
		return strings.Contains(name, input)
	}

	p := promptui.Select{
		Label:     prompt,
		Items:     packages,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
	}

	index, _, err := p.Run()
	if err != nil {
		return nil, err
	}

	return &packages[index], nil
}

// SelectSource prompts the user to select a package source.
func SelectSource(sources []string, prompt string) (string, error) {
	if len(sources) == 0 {
		return "", fmt.Errorf("no sources available")
	}

	if len(sources) == 1 {
		return sources[0], nil
	}

	p := promptui.Select{
		Label: prompt,
		Items: sources,
		Size:  10,
	}

	_, result, err := p.Run()
	if err != nil {
		return "", err
	}

	return result, nil
}

// Input prompts the user for text input.
func Input(prompt string, defaultValue string) (string, error) {
	p := promptui.Prompt{
		Label:   prompt,
		Default: defaultValue,
	}

	result, err := p.Run()
	if err != nil {
		return defaultValue, err
	}

	return result, nil
}

// SelectMultiple prompts the user to select multiple items.
// Note: promptui doesn't support multi-select out of the box,
// so we implement a simple version.
func SelectMultiple(items []string, prompt string) ([]string, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	fmt.Println(prompt)
	fmt.Println("Enter numbers separated by spaces (e.g., '1 3 5'), or 'all' for all items:")
	fmt.Println()

	for i, item := range items {
		fmt.Printf("  %d. %s\n", i+1, item)
	}

	fmt.Println()

	p := promptui.Prompt{
		Label: "Selection",
	}

	result, err := p.Run()
	if err != nil {
		return nil, err
	}

	result = strings.TrimSpace(result)
	if result == "" {
		return nil, nil
	}

	if strings.ToLower(result) == "all" {
		return items, nil
	}

	var selected []string
	for _, part := range strings.Fields(result) {
		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err == nil {
			if idx >= 1 && idx <= len(items) {
				selected = append(selected, items[idx-1])
			}
		}
	}

	return selected, nil
}
