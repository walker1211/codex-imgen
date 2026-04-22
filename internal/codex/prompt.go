package codex

import "strings"

func BuildPrompt(prelude string, prefix string, userPrompt string) string {
	trimmedPrefix := strings.TrimSpace(prefix)
	trimmedPrompt := strings.TrimSpace(userPrompt)

	commandPrompt := trimmedPrompt
	if trimmedPrefix != "" {
		commandPrompt = trimmedPrefix + " " + trimmedPrompt
	}

	prelude = strings.TrimSpace(prelude)
	if prelude == "" {
		return commandPrompt
	}
	return prelude + "\n\n" + commandPrompt
}
