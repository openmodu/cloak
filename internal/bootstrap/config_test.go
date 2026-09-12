package bootstrap

import (
	"context"
	"github.com/openmodu/cloak/internal/types"
	"testing"
)

func TestStartupMaskConfig(t *testing.T) {
	yes, no := true, false
	app, err := InitApp(Config{MaskConfig: map[string]*bool{"maskAll": &no, "maskAddress": &yes}})
	if err != nil {
		t.Fatal(err)
	}
	if !app.Conf.Mask().Enabled(types.EntityPhysicalAddress) || app.Conf.Mask().Enabled(types.EntityEmailAddress) {
		t.Fatal("startup config not applied")
	}
	text := "mail alice@example.com"
	got, _, err := app.Masker.Mask(context.Background(), text)
	if err != nil || got != text {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
