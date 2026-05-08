package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	// PacketHeaderSize is the size prefix at the front of every rnPacket frame.
	PacketHeaderSize = 2
	// PacketSubHeaderSize is the packed opcode/type + dummy + sequence section.
	PacketSubHeaderSize = 8
	// FrameHeaderSize is the full header size on the wire.
	FrameHeaderSize = PacketHeaderSize + PacketSubHeaderSize
)

var (
	ErrFrameTooShort    = errors.New("protocol: frame too short")
	ErrInvalidFrameSize = errors.New("protocol: invalid frame size")
)

type Opcode uint16

type PacketType uint16

const (
	PacketTypeNone     PacketType = 0
	PacketTypeRequest  PacketType = 1
	PacketTypeResponse PacketType = 2
	PacketTypeUpdate   PacketType = 3
)

func (t PacketType) String() string {
	switch t {
	case PacketTypeNone:
		return "none"
	case PacketTypeRequest:
		return "request"
	case PacketTypeResponse:
		return "response"
	case PacketTypeUpdate:
		return "update"
	default:
		return fmt.Sprintf("PacketType(%d)", t)
	}
}

type SubHeader struct {
	Index    Opcode
	Type     PacketType
	Dummy    uint32
	Sequence uint16
}

func (h SubHeader) MarshalBinary() ([]byte, error) {
	if h.Index > 0x3FFF {
		return nil, fmt.Errorf("protocol: opcode %d exceeds 14-bit field", h.Index)
	}
	if h.Type > 0x3 {
		return nil, fmt.Errorf("protocol: packet type %d exceeds 2-bit field", h.Type)
	}

	out := make([]byte, PacketSubHeaderSize)
	packed := uint16(h.Type)<<14 | (uint16(h.Index) & 0x3FFF)
	binary.LittleEndian.PutUint16(out[0:2], packed)
	binary.LittleEndian.PutUint32(out[2:6], h.Dummy)
	binary.LittleEndian.PutUint16(out[6:8], h.Sequence)
	return out, nil
}

func ParseSubHeader(data []byte) (SubHeader, error) {
	if len(data) < PacketSubHeaderSize {
		return SubHeader{}, ErrFrameTooShort
	}

	packed := binary.LittleEndian.Uint16(data[0:2])
	return SubHeader{
		Index:    Opcode(packed & 0x3FFF),
		Type:     PacketType(packed >> 14),
		Dummy:    binary.LittleEndian.Uint32(data[2:6]),
		Sequence: binary.LittleEndian.Uint16(data[6:8]),
	}, nil
}
