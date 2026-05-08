package protocol

import (
	"encoding/binary"
	"fmt"
)

// Frame models the on-wire rnPacket format used by the client.
//
// Wire layout:
//
//	[2-byte little-endian size][8-byte subheader][payload...]
//
// The size field stores the number of bytes after the size field itself,
// which means: PacketSubHeaderSize + len(payload).
type Frame struct {
	SubHeader SubHeader
	Payload   []byte
}

func (f Frame) SizeField() uint16 {
	return uint16(PacketSubHeaderSize + len(f.Payload))
}

func (f Frame) WireSize() int {
	return PacketHeaderSize + int(f.SizeField())
}

func (f Frame) MarshalBinary() ([]byte, error) {
	subHeader, err := f.SubHeader.MarshalBinary()
	if err != nil {
		return nil, err
	}
	size := f.SizeField()
	if size < PacketSubHeaderSize {
		return nil, ErrInvalidFrameSize
	}

	out := make([]byte, PacketHeaderSize+len(subHeader)+len(f.Payload))
	binary.LittleEndian.PutUint16(out[0:2], size)
	copy(out[2:10], subHeader)
	copy(out[10:], f.Payload)
	return out, nil
}

func ParseFrame(data []byte) (Frame, error) {
	if len(data) < FrameHeaderSize {
		return Frame{}, ErrFrameTooShort
	}

	size := int(binary.LittleEndian.Uint16(data[0:2]))
	if size < PacketSubHeaderSize {
		return Frame{}, fmt.Errorf("%w: size field %d smaller than subheader", ErrInvalidFrameSize, size)
	}
	if len(data) != PacketHeaderSize+size {
		return Frame{}, fmt.Errorf("%w: want %d bytes, got %d", ErrInvalidFrameSize, PacketHeaderSize+size, len(data))
	}

	subHeader, err := ParseSubHeader(data[2:10])
	if err != nil {
		return Frame{}, err
	}

	payload := make([]byte, size-PacketSubHeaderSize)
	copy(payload, data[10:])

	return Frame{
		SubHeader: subHeader,
		Payload:   payload,
	}, nil
}

func SplitFrames(stream []byte) (frames [][]byte, remainder []byte, err error) {
	cursor := 0
	for {
		if len(stream[cursor:]) < PacketHeaderSize {
			break
		}
		size := int(binary.LittleEndian.Uint16(stream[cursor : cursor+2]))
		if size < PacketSubHeaderSize {
			return nil, nil, fmt.Errorf("%w: size field %d smaller than subheader", ErrInvalidFrameSize, size)
		}
		frameSize := PacketHeaderSize + size
		if len(stream[cursor:]) < frameSize {
			break
		}
		frame := make([]byte, frameSize)
		copy(frame, stream[cursor:cursor+frameSize])
		frames = append(frames, frame)
		cursor += frameSize
	}

	remainder = make([]byte, len(stream)-cursor)
	copy(remainder, stream[cursor:])
	return frames, remainder, nil
}

func BuildPacket(index Opcode, packetType PacketType, seq uint16, payload []byte) ([]byte, error) {
	return Frame{
		SubHeader: SubHeader{
			Index:    index,
			Type:     packetType,
			Sequence: seq,
		},
		Payload: payload,
	}.MarshalBinary()
}

// BuildTextCommand wraps an ASCII command string in a zeroed subheader frame.
// This matches the client-side SendNetMessage() path used during login,
// character select, and some control-plane commands.
func BuildTextCommand(text string) ([]byte, error) {
	return Frame{Payload: []byte(text)}.MarshalBinary()
}

func IsTextCommand(frame Frame) bool {
	return frame.SubHeader.Index == 0 && frame.SubHeader.Type == PacketTypeNone
}
