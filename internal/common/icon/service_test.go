package icon

import (
	"bytes"
	"testing"
)

func TestCompressDecompressRoundtrip(t *testing.T) {
	testData := [][]byte{
		[]byte("simple icon data"),
		{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
		bytes.Repeat([]byte{0xFF}, 10000),
		{},
	}

	for i, original := range testData {
		compressed, err := compressIcon(original)
		if err != nil {
			t.Fatalf("case %d: compressIcon failed: %v", i, err)
		}

		decompressed, err := decompressIcon(compressed)
		if err != nil {
			t.Fatalf("case %d: decompressIcon failed: %v", i, err)
		}

		if !bytes.Equal(decompressed, original) {
			t.Errorf("case %d: roundtrip mismatch: got %d bytes, want %d bytes", i, len(decompressed), len(original))
		}
	}
}
