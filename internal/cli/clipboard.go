package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type clipboardCommand struct {
	name string
	args []string
}

func copyToClipboard(value string) error {
	var unavailable []string
	for _, candidate := range clipboardCommands(runtime.GOOS) {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			unavailable = append(unavailable, candidate.name)
			continue
		}
		command := exec.Command(path, candidate.args...)
		command.Stdin = strings.NewReader(value)
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", candidate.name, err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	return fmt.Errorf("no clipboard command found (tried %s)", strings.Join(unavailable, ", "))
}

func clipboardCommands(goos string) []clipboardCommand {
	switch goos {
	case "darwin":
		return []clipboardCommand{{name: "pbcopy"}}
	case "windows":
		return []clipboardCommand{{name: "cmd", args: []string{"/c", "clip"}}}
	default:
		return []clipboardCommand{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		}
	}
}
