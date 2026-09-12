package nerrepo

import (
	"context"
	"testing"
)

func TestOpenCCConversion(t *testing.T) {
	c, err := NewOpenCC()
	if err != nil {
		t.Skip("OpenCC is not installed")
	}
	text, err := c.ToModel(context.Background(), "北京市朝阳区")
	if err != nil || text != "北京市朝陽區" {
		t.Fatalf("text=%q err=%v", text, err)
	}
	tokens, err := c.ToSource(context.Background(), []string{"朝", "陽", "區"})
	if err != nil || len(tokens) != 3 || tokens[1] != "阳" || tokens[2] != "区" {
		t.Fatalf("tokens=%v err=%v", tokens, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ToModel(ctx, "中文"); err == nil {
		t.Fatal("ignored cancellation")
	}
}
