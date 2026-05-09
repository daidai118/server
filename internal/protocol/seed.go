package protocol

import (
	"crypto/cipher"
	"fmt"

	"laghaim-go/internal/protocol/seedkisa"
)

var DefaultSeedUserKey = [32]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
}

type SeedCodec struct {
	block cipher.Block
}

func NewSeedCodec(key []byte) (SeedCodec, error) {
	block, err := seedkisa.NewCipher(key)
	if err != nil {
		return SeedCodec{}, fmt.Errorf("protocol: create SEED cipher: %w", err)
	}
	return SeedCodec{block: block}, nil
}

func MustNewDefaultSeedCodec() SeedCodec {
	codec, err := NewSeedCodec(DefaultSeedUserKey[:])
	if err != nil {
		panic(err)
	}
	return codec
}

func (c SeedCodec) EncryptFrame(frame []byte) ([]byte, error) {
	return c.transformFrame(frame, true)
}

func (c SeedCodec) DecryptFrame(frame []byte) ([]byte, error) {
	return c.transformFrame(frame, false)
}

func (c SeedCodec) transformFrame(frame []byte, encrypt bool) ([]byte, error) {
	if c.block == nil {
		return nil, fmt.Errorf("protocol: seed codec not initialized")
	}
	if len(frame) < PacketHeaderSize {
		return nil, ErrFrameTooShort
	}

	out := make([]byte, len(frame))
	copy(out, frame)

	payload := out[PacketHeaderSize:]
	fullBlocks := len(payload) / c.block.BlockSize()
	for i := 0; i < fullBlocks; i++ {
		start := i * c.block.BlockSize()
		end := start + c.block.BlockSize()
		if encrypt {
			c.block.Encrypt(payload[start:end], payload[start:end])
		} else {
			c.block.Decrypt(payload[start:end], payload[start:end])
		}
	}
	return out, nil
}
