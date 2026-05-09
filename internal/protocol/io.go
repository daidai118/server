package protocol

import (
	"fmt"
	"io"
)

type FrameCipher interface {
	EncryptFrame(frame []byte) ([]byte, error)
	DecryptFrame(frame []byte) ([]byte, error)
}

func ReadFrame(r io.Reader, cipher FrameCipher) (Frame, error) {
	header := make([]byte, PacketHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return Frame{}, err
	}
	size := int(header[0]) | int(header[1])<<8
	if size < PacketSubHeaderSize {
		return Frame{}, fmt.Errorf("%w: size field %d smaller than subheader", ErrInvalidFrameSize, size)
	}

	rest := make([]byte, size)
	if _, err := io.ReadFull(r, rest); err != nil {
		return Frame{}, err
	}
	raw := append(header, rest...)
	if cipher != nil {
		var err error
		raw, err = cipher.DecryptFrame(raw)
		if err != nil {
			return Frame{}, err
		}
	}
	return ParseFrame(raw)
}

func WriteFrame(w io.Writer, cipher FrameCipher, frame Frame) error {
	raw, err := frame.MarshalBinary()
	if err != nil {
		return err
	}
	if cipher != nil {
		raw, err = cipher.EncryptFrame(raw)
		if err != nil {
			return err
		}
	}
	_, err = w.Write(raw)
	return err
}

func WriteTextCommand(w io.Writer, cipher FrameCipher, text string) error {
	raw, err := BuildTextCommand(text)
	if err != nil {
		return err
	}
	if cipher != nil {
		raw, err = cipher.EncryptFrame(raw)
		if err != nil {
			return err
		}
	}
	_, err = w.Write(raw)
	return err
}
