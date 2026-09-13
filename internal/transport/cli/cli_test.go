package cli

import (
	"bytes"
	"strings"
	"testing"
	"unicode"
)

func TestExportRestore(t *testing.T) {
	var masked, restored, errs bytes.Buffer
	input := "mail alice@example.com"
	if code := Run([]string{"mask", "--json"}, strings.NewReader(input), &masked, &errs); code != 0 {
		t.Fatal(errs.String())
	}
	if code := Run([]string{"restore"}, &masked, &restored, &errs); code != 0 {
		t.Fatal(errs.String())
	}
	if restored.String() != input {
		t.Fatalf("%q", restored.String())
	}
}
func TestHelpDoesNotReadStdin(t *testing.T) {
	var out bytes.Buffer
	if code := Run([]string{"--help"}, nil, &out, &out); code != 0 {
		t.Fatal(code)
	}
}

func TestCLIEnglishMessages(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no arguments", nil, "Usage:"},
		{"help", []string{"--help"}, "Reads from standard input"},
		{"command help", []string{"mask", "--help"}, "Input file"},
		{"unknown command", []string{"unknown"}, "Unknown command:"},
		{"round trip", []string{"roundtrip"}, "Round trip verified:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			Run(tc.args, strings.NewReader("alice@example.com"), &out, &out)
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("missing %q in %q", tc.want, out.String())
			}
			for _, r := range out.String() {
				if unicode.Is(unicode.Han, r) {
					t.Fatalf("unexpected Chinese CLI message: %s", out.String())
				}
			}
		})
	}
}
