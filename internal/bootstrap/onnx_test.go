//go:build cloak_onnx

package bootstrap_test

import (
	"context"
	"github.com/openmodu/cloak/internal/repo/nerrepo"
	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/pkg/onnxrt"
	"os"
	"path/filepath"
	"testing"
)

// Unlike production's optional NER loading, this check fails on missing models.
func TestRealONNX(t *testing.T) {
	root := os.Getenv("CLOAK_TEST_MODELS_DIR")
	if root == "" {
		t.Skip("set CLOAK_TEST_MODELS_DIR to exercise real models")
	}
	for _, tc := range []struct {
		id, text string
		lang     types.Language
	}{
		{"funstory-ai/neurobert-mini", "John Smith works at Microsoft in New York.", types.LangEnglish},
		{"ckiplab/bert-tiny-chinese-ner", "王小明住在北京市朝阳区建国路88号。", types.LangZhHans},
	} {
		t.Run(tc.id, func(t *testing.T) {
			dir := filepath.Join(root, tc.id)
			session, err := onnxrt.Open(filepath.Join(dir, "onnx", "model_quantized.onnx"))
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			var opts []nerrepo.Option
			if tc.lang.IsChinese() {
				c, err := nerrepo.NewOpenCC()
				if err != nil {
					t.Fatal(err)
				}
				opts = append(opts, nerrepo.WithConverter(c))
			}
			r, err := nerrepo.LoadFromDir(tc.id, dir, session, opts...)
			if err != nil {
				t.Fatal(err)
			}
			spans, err := r.Recognize(context.Background(), tc.text, tc.lang)
			if err != nil {
				t.Fatal(err)
			}
			if len(spans) == 0 {
				t.Fatal("model produced no entities for smoke sample")
			}
			for _, s := range spans {
				if !s.ValidIn(len(tc.text)) {
					t.Fatalf("invalid span %+v", s)
				}
				t.Logf("%s %d:%d %q %.4f", s.Type, s.Start, s.End, s.Text(tc.text), s.Score)
			}
		})
	}
}
