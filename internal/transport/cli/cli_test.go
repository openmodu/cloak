package cli

import (
	"bytes"
	"strings"
	"testing"
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
