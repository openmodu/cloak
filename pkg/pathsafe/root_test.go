package pathsafe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootContainmentAndPinning(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "keys")
	if err := os.Mkdir(base, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "key"), []byte("inside"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Symlink(outside, filepath.Join(base, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside", outside, "escape"} {
		if b, err := ReadFile(root, name); err == nil {
			t.Fatalf("escaped %s: %q", name, b)
		}
	}
	if err := os.Rename(base, base+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(base, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "key"), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := ReadFile(root, "key")
	if err != nil || string(b) != "inside" {
		t.Fatalf("root not pinned: %q %v", b, err)
	}
}
