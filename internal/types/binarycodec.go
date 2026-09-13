package types

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
)

// EncodeBinary uses the little-endian matched-span binary format (20-byte records).
// Only matched slices are serialized, rather than the entire input document.
func (m *MaskMeta) EncodeBinary() (string, error) {
	if m == nil {
		m = &MaskMeta{}
	}
	textLen := uint64(0)
	for _, it := range m.Items {
		if !it.Type.Valid() || it.Start < 0 || it.End <= it.Start || it.End > len(m.Original) {
			return "", fmt.Errorf("invalid mask span")
		}
		textLen += uint64(it.End - it.Start)
	}
	total := 12 + textLen + uint64(len(m.Items))*20
	if total > math.MaxUint32 {
		return "", fmt.Errorf("mask metadata too large")
	}
	raw := make([]byte, int(total))
	binary.LittleEndian.PutUint32(raw, uint32(total))
	binary.LittleEndian.PutUint32(raw[4:], uint32(textLen))
	offset := (8 + int(textLen) + 3) &^ 3
	cursor := 0
	for _, it := range m.Items {
		n := copy(raw[8+cursor:], m.Original[it.Start:it.End])
		record := raw[offset : offset+20]
		binary.LittleEndian.PutUint32(record, it.ID)
		record[4] = byte(it.Type)
		binary.LittleEndian.PutUint32(record[8:], uint32(cursor))
		binary.LittleEndian.PutUint32(record[12:], uint32(cursor+n))
		binary.LittleEndian.PutUint32(record[16:], math.Float32bits(it.Score))
		cursor += n
		offset += 20
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func decodeBinary(raw []byte) (*MaskMeta, error) {
	invalid := func() (*MaskMeta, error) { return nil, fmt.Errorf("invalid binary mask metadata") }
	if len(raw) < 8 || uint64(binary.LittleEndian.Uint32(raw)) != uint64(len(raw)) {
		return invalid()
	}
	n := uint64(binary.LittleEndian.Uint32(raw[4:]))
	if 8+n > uint64(len(raw)) {
		return invalid()
	}
	offset := (8 + int(n) + 3) &^ 3
	if offset > len(raw) || (len(raw)-offset)%20 > 4 {
		return invalid()
	}
	m := &MaskMeta{Original: string(raw[8 : 8+int(n)])}
	for ; offset+20 <= len(raw); offset += 20 {
		r := raw[offset : offset+20]
		it := MaskedItem{ID: binary.LittleEndian.Uint32(r), Type: EntityType(r[4]), Start: int(binary.LittleEndian.Uint32(r[8:])), End: int(binary.LittleEndian.Uint32(r[12:])), Score: math.Float32frombits(binary.LittleEndian.Uint32(r[16:]))}
		if !it.Type.Valid() || it.Start < 0 || it.End <= it.Start || it.End > len(m.Original) {
			return invalid()
		}
		m.Items = append(m.Items, it)
	}
	return m, nil
}
