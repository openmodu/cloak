package regexrepo

import (
	"context"
	"github.com/openmodu/cloak/internal/types"
	"testing"
)

func TestUnicodeClasses(t *testing.T) {
	for _, tc := range []struct {
		text string
		typ  types.EntityType
		want string
	}{
		{"１２３４５６７８９０", types.EntityPhoneNumber, "１２３４５６７８９０"},
		{"verification\u00a0code: AB12", types.EntityVerificationCode, "AB12"},
		{"pwd:\u00a0secret\u00a0tail", types.EntityPassword, "secret"},
		{"a\u03011234", types.EntityVerificationCode, ""},
		{"a\u200c1234", types.EntityVerificationCode, ""},
	} {
		t.Run(tc.text, func(t *testing.T) {
			r, err := New(tc.typ, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			spans, err := r.Recognize(context.Background(), tc.text, types.LangEnglish)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(spans) != 0 {
					t.Fatalf("unexpected %v", spans)
				}
				return
			}
			if len(spans) == 0 || spans[0].Text(tc.text) != tc.want {
				t.Fatalf("got %v, want %q", spans, tc.want)
			}
		})
	}
}
