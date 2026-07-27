package cli

import (
	"reflect"
	"testing"
)

func TestClipboardCommands(t *testing.T) {
	tests := map[string][]clipboardCommand{
		"darwin":  {{name: "pbcopy"}},
		"windows": {{name: "cmd", args: []string{"/c", "clip"}}},
		"linux": {
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		},
	}
	for goos, wanted := range tests {
		if got := clipboardCommands(goos); !reflect.DeepEqual(got, wanted) {
			t.Fatalf("clipboardCommands(%q) = %#v, want %#v", goos, got, wanted)
		}
	}
}
