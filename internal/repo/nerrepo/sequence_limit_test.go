package nerrepo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenizerSequenceLimit(t *testing.T) {
	for _, tc := range []struct {
		limit string
		want  int
	}{{"16", 16}, {"256", 128}, {"1000000000000000019884624838656", 128}} {
		dir := writeModelDir(t, `{`+labelsJSON+`,"max_position_embeddings":128}`)
		if err := os.WriteFile(filepath.Join(dir, "tokenizer_config.json"), []byte(`{"model_max_length":`+tc.limit+`}`), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadModelConfig(dir)
		if err != nil || cfg.MaxSeqLen != tc.want {
			t.Fatalf("%s: %+v %v", tc.limit, cfg, err)
		}
		if _, err := LoadFromDir("test", dir, nil, WithMaxSeqLen(tc.want+1)); err == nil {
			t.Fatal("override exceeded limit")
		}
	}
}
