package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestSeedCodecMatchesClientReferenceVector(t *testing.T) {
	// Reference ciphertext generated from the client SEED_256_KISA.cpp implementation
	// with key bytes 0x00..0x1f and plaintext bytes 0x00..0x1f.
	codec := MustNewDefaultSeedCodec()

	frame := make([]byte, 34)
	frame[0] = 0x20
	frame[1] = 0x00
	for i := 0; i < 32; i++ {
		frame[2+i] = byte(i)
	}

	encrypted, err := codec.EncryptFrame(frame)
	if err != nil {
		t.Fatalf("EncryptFrame() error = %v", err)
	}

	got := hex.EncodeToString(encrypted[2:])
	const want = "1f3fca1b58dae181580a6aa2158481e431da9073e702a343a97e745bf9b77f51"
	if got != want {
		t.Fatalf("ciphertext mismatch:\n got: %s\nwant: %s", got, want)
	}

	decrypted, err := codec.DecryptFrame(encrypted)
	if err != nil {
		t.Fatalf("DecryptFrame() error = %v", err)
	}
	if !bytes.Equal(decrypted, frame) {
		t.Fatalf("roundtrip mismatch: got %x want %x", decrypted, frame)
	}
}

func TestSeedCodecLeavesTrailingPartialBlockUntouched(t *testing.T) {
	codec := MustNewDefaultSeedCodec()

	frame := []byte{0x13, 0x00}
	frame = append(frame, bytes.Repeat([]byte{0xAA}, 17)...)

	encrypted, err := codec.EncryptFrame(frame)
	if err != nil {
		t.Fatalf("EncryptFrame() error = %v", err)
	}

	if encrypted[len(encrypted)-1] != frame[len(frame)-1] {
		t.Fatalf("last byte should remain untouched for trailing partial block: got %x want %x", encrypted[len(encrypted)-1], frame[len(frame)-1])
	}

	decrypted, err := codec.DecryptFrame(encrypted)
	if err != nil {
		t.Fatalf("DecryptFrame() error = %v", err)
	}
	if !bytes.Equal(decrypted, frame) {
		t.Fatalf("roundtrip mismatch: got %x want %x", decrypted, frame)
	}
}
