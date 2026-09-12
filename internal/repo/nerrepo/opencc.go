package nerrepo

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// OpenCC converts via stdin, keeping sensitive text out of process arguments.
// Each call owns its process; token conversion is batched per inference.
type OpenCC struct{ path string }

func NewOpenCC() (*OpenCC, error) {
	path, err := exec.LookPath("opencc")
	if err != nil {
		return nil, err
	}
	return &OpenCC{path: path}, nil
}

func (c *OpenCC) convert(ctx context.Context, text, config string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.path, "-c", config)
	cmd.Stdin = strings.NewReader(text)
	raw, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("OpenCC %s conversion failed: %w", config, err)
	}
	return string(raw), nil
}
func (c *OpenCC) ToModel(ctx context.Context, text string) (string, error) {
	return c.convert(ctx, text, "s2t")
}
func (c *OpenCC) ToSource(ctx context.Context, tokens []string) ([]string, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	text, err := c.convert(ctx, strings.Join(tokens, "\n"), "t2s")
	if err != nil {
		return nil, err
	}
	out := strings.Split(text, "\n")
	if len(out) != len(tokens) {
		return nil, fmt.Errorf("OpenCC changed token boundaries")
	}
	return out, nil
}
