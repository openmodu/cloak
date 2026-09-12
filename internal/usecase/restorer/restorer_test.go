package restorer

import (
	"context"
	"testing"

	"github.com/openmodu/cloak/internal/types"
)

func meta() *types.MaskMeta {
	return &types.MaskMeta{
		Original: "mail a@b.io tel 12345678",
		Items: []types.MaskedItem{
			{ID: 1, Type: types.EntityEmailAddress, Start: 5, End: 11},
			{ID: 2, Type: types.EntityPhoneNumber, Start: 16, End: 24},
		},
	}
}

func TestRestoreBasic(t *testing.T) {
	got, err := New().Restore(context.Background(), "mail __PII_EMAIL_ADDRESS_1__ tel __PII_PHONE_NUMBER_2__", meta())
	if err != nil {
		t.Fatal(err)
	}
	if want := "mail a@b.io tel 12345678"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRestoreHandlesReorder(t *testing.T) {
	// 大模型调换语序后，按占位符在文本里的实际位置重建
	in := "tel __PII_PHONE_NUMBER_2__, mail __PII_EMAIL_ADDRESS_1__"
	got, err := New().Restore(context.Background(), in, meta())
	if err != nil {
		t.Fatal(err)
	}
	if want := "tel 12345678, mail a@b.io"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// 还原按凭据逐条查找占位符的第一次出现，因此同一个占位符重复出现时只还原第一处。
// 这是刻意的：凭据里一个编号只对应原文里的一个区间，重复出现多半是模型自己复制的，
// 全部替换反而会把模型的复述也改掉。
func TestRestoreOnlyFirstOccurrence(t *testing.T) {
	in := "mail __PII_EMAIL_ADDRESS_1__ (again __PII_EMAIL_ADDRESS_1__)"
	got, err := New().Restore(context.Background(), in, meta())
	if err != nil {
		t.Fatal(err)
	}
	if want := "mail a@b.io (again __PII_EMAIL_ADDRESS_1__)"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRestoreKeepsUnknownPlaceholder(t *testing.T) {
	// 编号不在凭据里，或标签与编号对不上，都原样保留而不是丢弃文本
	in := "x __PII_EMAIL_ADDRESS_9__ y __PII_PHONE_NUMBER_1__"
	got, err := New().Restore(context.Background(), in, meta())
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Fatalf("got %q, want unchanged", got)
	}
}

func TestRestoreMissingPlaceholderIsNotAnError(t *testing.T) {
	// 模型吞掉了一个占位符：能还原的照常还原
	got, err := New().Restore(context.Background(), "mail __PII_EMAIL_ADDRESS_1__", meta())
	if err != nil {
		t.Fatal(err)
	}
	if want := "mail a@b.io"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRestoreNilMeta(t *testing.T) {
	got, err := New().Restore(context.Background(), "unchanged", nil)
	if err != nil || got != "unchanged" {
		t.Fatalf("got %q %v", got, err)
	}
}
