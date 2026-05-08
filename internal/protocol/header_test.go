package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestSubHeaderRoundTrip(t *testing.T) {
	original := SubHeader{
		Index:    ReqCharWalk,
		Type:     PacketTypeRequest,
		Dummy:    0x12345678,
		Sequence: 42,
	}

	encoded, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}

	decoded, err := ParseSubHeader(encoded)
	if err != nil {
		t.Fatalf("ParseSubHeader() error = %v", err)
	}

	if decoded != original {
		t.Fatalf("decoded subheader mismatch: got %+v want %+v", decoded, original)
	}
}

func TestBuildPacketMatchesClientSizeSemantics(t *testing.T) {
	payload := []byte{0xAA, 0xBB, 0xCC, 0xDD}

	raw, err := BuildPacket(ReqGameStart, PacketTypeRequest, 7, payload)
	if err != nil {
		t.Fatalf("BuildPacket() error = %v", err)
	}

	if got, want := len(raw), PacketHeaderSize+PacketSubHeaderSize+len(payload); got != want {
		t.Fatalf("wire size mismatch: got %d want %d", got, want)
	}

	if got, want := binary.LittleEndian.Uint16(raw[0:2]), uint16(PacketSubHeaderSize+len(payload)); got != want {
		t.Fatalf("size field mismatch: got %d want %d", got, want)
	}

	frame, err := ParseFrame(raw)
	if err != nil {
		t.Fatalf("ParseFrame() error = %v", err)
	}

	if frame.SubHeader.Index != ReqGameStart || frame.SubHeader.Type != PacketTypeRequest || frame.SubHeader.Sequence != 7 {
		t.Fatalf("unexpected subheader: %+v", frame.SubHeader)
	}
	if !bytes.Equal(frame.Payload, payload) {
		t.Fatalf("payload mismatch: got %x want %x", frame.Payload, payload)
	}
}

func TestBuildTextCommandUsesZeroedSubHeader(t *testing.T) {
	raw, err := BuildTextCommand("login\n")
	if err != nil {
		t.Fatalf("BuildTextCommand() error = %v", err)
	}

	frame, err := ParseFrame(raw)
	if err != nil {
		t.Fatalf("ParseFrame() error = %v", err)
	}

	if !IsTextCommand(frame) {
		t.Fatalf("frame should be treated as text command: %+v", frame.SubHeader)
	}
	if got, want := string(frame.Payload), "login\n"; got != want {
		t.Fatalf("payload mismatch: got %q want %q", got, want)
	}
}

func TestSplitFrames(t *testing.T) {
	first, err := BuildPacket(ReqPulse, PacketTypeRequest, 1, []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("BuildPacket(first) error = %v", err)
	}
	second, err := BuildTextCommand("char_new\n")
	if err != nil {
		t.Fatalf("BuildTextCommand(second) error = %v", err)
	}

	stream := append(append([]byte{}, first...), second...)
	stream = append(stream, 0xFF) // incomplete next frame fragment

	frames, rest, err := SplitFrames(stream)
	if err != nil {
		t.Fatalf("SplitFrames() error = %v", err)
	}

	if got, want := len(frames), 2; got != want {
		t.Fatalf("frame count mismatch: got %d want %d", got, want)
	}
	if got, want := len(rest), 1; got != want {
		t.Fatalf("remainder length mismatch: got %d want %d", got, want)
	}
}
