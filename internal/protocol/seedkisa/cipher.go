package seedkisa

import (
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"math/bits"
)

const BlockSize = 16

type KeySizeError int

func (k KeySizeError) Error() string {
	return fmt.Sprintf("seedkisa: invalid key size %d", int(k))
}

type Cipher struct {
	pdwRoundKey [48]uint32
}

var seed256rot = [...]int{12, 9, 9, 11, 11, 12}

func NewCipher(key []byte) (cipher.Block, error) {
	if len(key) != 32 {
		return nil, KeySizeError(len(key))
	}
	c := new(Cipher)
	c.expandKey(key)
	return c, nil
}

func (s *Cipher) BlockSize() int {
	return BlockSize
}

func (s *Cipher) Encrypt(dst, src []byte) {
	if len(src) < BlockSize {
		panic(fmt.Sprintf("seedkisa: invalid block size %d (src)", len(src)))
	}
	if len(dst) < BlockSize {
		panic(fmt.Sprintf("seedkisa: invalid block size %d (dst)", len(dst)))
	}
	s.encrypt(dst, src)
}

func (s *Cipher) Decrypt(dst, src []byte) {
	if len(src) < BlockSize {
		panic(fmt.Sprintf("seedkisa: invalid block size %d (src)", len(src)))
	}
	if len(dst) < BlockSize {
		panic(fmt.Sprintf("seedkisa: invalid block size %d (dst)", len(dst)))
	}
	s.decrypt(dst, src)
}

func (s *Cipher) encrypt(dst, src []byte) {
	L0 := endianChange(binary.LittleEndian.Uint32(src[0:]))
	L1 := endianChange(binary.LittleEndian.Uint32(src[4:]))
	R0 := endianChange(binary.LittleEndian.Uint32(src[8:]))
	R1 := endianChange(binary.LittleEndian.Uint32(src[12:]))

	for i := 0; i < 48; i += 4 {
		L0, L1 = seedRound(L0, L1, R0, R1, s.pdwRoundKey[i], s.pdwRoundKey[i+1])
		R0, R1 = seedRound(R0, R1, L0, L1, s.pdwRoundKey[i+2], s.pdwRoundKey[i+3])
	}

	L0 = endianChange(L0)
	L1 = endianChange(L1)
	R0 = endianChange(R0)
	R1 = endianChange(R1)

	binary.LittleEndian.PutUint32(dst[0:], R0)
	binary.LittleEndian.PutUint32(dst[4:], R1)
	binary.LittleEndian.PutUint32(dst[8:], L0)
	binary.LittleEndian.PutUint32(dst[12:], L1)
}

func (s *Cipher) decrypt(dst, src []byte) {
	L0 := endianChange(binary.LittleEndian.Uint32(src[0:]))
	L1 := endianChange(binary.LittleEndian.Uint32(src[4:]))
	R0 := endianChange(binary.LittleEndian.Uint32(src[8:]))
	R1 := endianChange(binary.LittleEndian.Uint32(src[12:]))

	for i := 46; i >= 0; i -= 4 {
		L0, L1 = seedRound(L0, L1, R0, R1, s.pdwRoundKey[i], s.pdwRoundKey[i+1])
		R0, R1 = seedRound(R0, R1, L0, L1, s.pdwRoundKey[i-2], s.pdwRoundKey[i-1])
	}

	L0 = endianChange(L0)
	L1 = endianChange(L1)
	R0 = endianChange(R0)
	R1 = endianChange(R1)

	binary.LittleEndian.PutUint32(dst[0:], R0)
	binary.LittleEndian.PutUint32(dst[4:], R1)
	binary.LittleEndian.PutUint32(dst[8:], L0)
	binary.LittleEndian.PutUint32(dst[12:], L1)
}

func (s *Cipher) expandKey(key []byte) {
	A := endianChange(binary.LittleEndian.Uint32(key[0:]))
	B := endianChange(binary.LittleEndian.Uint32(key[4:]))
	C := endianChange(binary.LittleEndian.Uint32(key[8:]))
	D := endianChange(binary.LittleEndian.Uint32(key[12:]))
	E := endianChange(binary.LittleEndian.Uint32(key[16:]))
	F := endianChange(binary.LittleEndian.Uint32(key[20:]))
	G := endianChange(binary.LittleEndian.Uint32(key[24:]))
	H := endianChange(binary.LittleEndian.Uint32(key[28:]))

	t0 := (((A + C) ^ E) - F) ^ kc[0]
	t1 := (((B - D) ^ G) + H) ^ kc[0]
	s.pdwRoundKey[0] = g(t0)
	s.pdwRoundKey[1] = g(t1)

	for i := 1; i < 24; i++ {
		rot := seed256rot[i%6]
		if (i+1)%2 == 0 {
			t0 = D
			D = (D >> rot) ^ (C << (32 - rot))
			C = (C >> rot) ^ (B << (32 - rot))
			B = (B >> rot) ^ (A << (32 - rot))
			A = (A >> rot) ^ (t0 << (32 - rot))
		} else {
			t0 = E
			E = (E << rot) ^ (F >> (32 - rot))
			F = (F << rot) ^ (G >> (32 - rot))
			G = (G << rot) ^ (H >> (32 - rot))
			H = (H << rot) ^ (t0 >> (32 - rot))
		}
		t0 = (((A + C) ^ E) - F) ^ kc[i]
		t1 = (((B - D) ^ G) + H) ^ kc[i]
		s.pdwRoundKey[i*2] = g(t0)
		s.pdwRoundKey[i*2+1] = g(t1)
	}
}

func g(n uint32) uint32 {
	return ss0[0xFF&(n>>0)] ^ ss1[0xFF&(n>>8)] ^ ss2[0xFF&(n>>16)] ^ ss3[0xFF&(n>>24)]
}

func endianChange(v uint32) uint32 {
	return bits.ReverseBytes32(v)
}

func seedRound(l0, l1, r0, r1, k0, k1 uint32) (uint32, uint32) {
	t0 := r0 ^ k0
	t1 := r1 ^ k1
	t1 ^= t0
	t1 = g(t1)
	t0 += t1
	t0 = g(t0)
	t1 += t0
	t1 = g(t1)
	t0 += t1
	l0 ^= t0
	l1 ^= t1
	return l0, l1
}
