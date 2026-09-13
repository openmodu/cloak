package types

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestBinaryBinaryFixture(t *testing.T) {
	// u32 total=35, text=3, "abc", alignment, id=1, type=email,
	// start=0, end=3, score=1.0, 3 trailing allocation bytes.
	raw, err := hex.DecodeString("230000000300000061626300010000000200000000000000030000000000803f000000")
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	m, err := DecodeMaskMeta(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if text, ok := m.Lookup(1); !ok || text != "abc" {
		t.Fatalf("%+v", m)
	}
	got, err := m.EncodeBinary()
	if err != nil || got != encoded {
		t.Fatalf("encoded=%s err=%v", got, err)
	}
	old, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeMaskMeta(old); err != nil {
		t.Fatal(err)
	}
	raw[0]++
	if _, err = DecodeMaskMeta(base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("accepted invalid length")
	}
}

func TestBinaryOnlySerializesMatchedText(t *testing.T) {
	m := &MaskMeta{Original: "prefix secret suffix", Items: []MaskedItem{{ID: 1, Type: EntityPassword, Start: 7, End: 13, Score: 1}}}
	encoded, err := m.EncodeBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeMaskMeta(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Original != "secret" {
		t.Fatalf("unexpected retained text %q", decoded.Original)
	}
	empty, err := (&MaskMeta{}).EncodeBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeMaskMeta(empty); err != nil {
		t.Fatal(err)
	}
}
