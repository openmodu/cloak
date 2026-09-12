package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openmodu/cloak/internal/bootstrap"
	"github.com/openmodu/cloak/internal/types"
	"io"
)

type jsonText struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
	MaskMeta string `json:"maskMeta,omitempty"`
}

func runJSON(ctx context.Context, app *bootstrap.App, cmd, input, language string, out io.Writer) error {
	var items []jsonText
	if cmd == "restore" {
		var item jsonText
		if err := json.Unmarshal([]byte(input), &item); err != nil {
			return err
		}
		items = []jsonText{item}
	} else if err := json.Unmarshal([]byte(input), &items); err != nil {
		return err
	}
	results := make([]jsonText, 0, len(items))
	for i, item := range items {
		var result jsonText
		if cmd == "mask-batch" {
			lang := item.Language
			if lang == "" {
				lang = language
			}
			text, meta, err := app.Masker.MaskWithLanguage(ctx, item.Text, types.Language(lang))
			if err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
			encoded, err := meta.EncodeAIFW()
			if err != nil {
				return err
			}
			result = jsonText{Text: text, MaskMeta: encoded}
		} else {
			meta, err := types.DecodeMaskMeta(item.MaskMeta)
			if err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
			text, err := app.Restorer.Restore(ctx, item.Text, meta)
			if err != nil {
				return err
			}
			result.Text = text
		}
		results = append(results, result)
	}
	if cmd == "restore" {
		_, err := fmt.Fprint(out, results[0].Text)
		return err
	}
	return json.NewEncoder(out).Encode(results)
}
