package legacyxor

import (
	"bytes"
	"testing"
)

func TestTransformIsSymmetric(t *testing.T) {
	codec := NewPythonReference(LoginServerKey, 0x11)
	plain := []byte("login\nuser\npassword d n 0\n")

	encrypted := codec.Transform(plain)
	decrypted := codec.Transform(encrypted)

	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("roundtrip mismatch: got %q want %q", decrypted, plain)
	}
}

func TestTransformUsesDifferentPrivateKeys(t *testing.T) {
	loginCodec := NewPythonReference(LoginServerKey, 0)
	zoneCodec := NewPythonReference(ZoneServerKey, 0)
	plain := []byte{1, 2, 3, 4, 5}

	loginCipher := loginCodec.Transform(plain)
	zoneCipher := zoneCodec.Transform(plain)

	if bytes.Equal(loginCipher, zoneCipher) {
		t.Fatal("ciphertext should differ when private keys differ")
	}
}
