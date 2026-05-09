package protocol

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodeRequestCharWalk(t *testing.T) {
	payload := make([]byte, 9)
	binary.LittleEndian.PutUint32(payload[0:4], math.Float32bits(33000.5))
	binary.LittleEndian.PutUint32(payload[4:8], math.Float32bits(34000.25))
	payload[8] = 1

	packet, err := DecodeRequestCharWalk(payload)
	if err != nil {
		t.Fatalf("DecodeRequestCharWalk() error = %v", err)
	}
	if packet.PosX != 33000.5 || packet.PosZ != 34000.25 || !packet.Run {
		t.Fatalf("unexpected walk packet: %+v", packet)
	}
}

func TestDecodeRequestCharPlace(t *testing.T) {
	payload := make([]byte, 18)
	payload[0] = 2
	binary.LittleEndian.PutUint32(payload[1:5], math.Float32bits(12.5))
	binary.LittleEndian.PutUint32(payload[5:9], math.Float32bits(22.25))
	binary.LittleEndian.PutUint32(payload[9:13], math.Float32bits(1.5))
	binary.LittleEndian.PutUint32(payload[13:17], uint32(7))
	payload[17] = 1

	packet, err := DecodeRequestCharPlace(payload)
	if err != nil {
		t.Fatalf("DecodeRequestCharPlace() error = %v", err)
	}
	if packet.CharType != 2 || packet.PosX != 12.5 || packet.PosZ != 22.25 || packet.Direction != 1.5 || packet.RemainFrame != 7 || !packet.Run {
		t.Fatalf("unexpected place packet: %+v", packet)
	}
}

func TestDecodeRequestCharStop(t *testing.T) {
	payload := make([]byte, 13)
	payload[0] = 3
	binary.LittleEndian.PutUint32(payload[1:5], math.Float32bits(55.5))
	binary.LittleEndian.PutUint32(payload[5:9], math.Float32bits(66.75))
	binary.LittleEndian.PutUint32(payload[9:13], math.Float32bits(2.75))

	packet, err := DecodeRequestCharStop(payload)
	if err != nil {
		t.Fatalf("DecodeRequestCharStop() error = %v", err)
	}
	if packet.CharType != 3 || packet.PosX != 55.5 || packet.PosZ != 66.75 || packet.Direction != 2.75 {
		t.Fatalf("unexpected stop packet: %+v", packet)
	}
}

func TestDecodeItemAndWearRequests(t *testing.T) {
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload[0:4], 1234)

	wear, err := DecodeRequestWear(payload)
	if err != nil {
		t.Fatalf("DecodeRequestWear() error = %v", err)
	}
	if wear.WearWhere != 1234 {
		t.Fatalf("unexpected wear packet: %+v", wear)
	}

	drop, err := DecodeRequestItemDrop(payload[:4])
	if err != nil {
		t.Fatalf("DecodeRequestItemDrop() error = %v", err)
	}
	if drop.ItemIndex != 1234 {
		t.Fatalf("unexpected drop packet: %+v", drop)
	}

	pick, err := DecodeRequestItemPick(payload[:4])
	if err != nil {
		t.Fatalf("DecodeRequestItemPick() error = %v", err)
	}
	if pick.ItemIndex != 1234 {
		t.Fatalf("unexpected pick packet: %+v", pick)
	}
}

func TestTypedPacketPayloadSizes(t *testing.T) {
	mapInChar := UpdateMapInChar{
		Type:        0,
		CharacterID: 1001,
		Name:        "Alice",
		Race:        2,
		Sex:         0,
		Hair:        1,
		PosX:        33000,
		PosZ:        33100,
		Direction:   1.25,
		Vital:       100,
	}
	if got, want := len(mapInChar.Frame().Payload), 211; got != want {
		t.Fatalf("UpdateMapInChar payload size = %d, want %d", got, want)
	}

	mapInNPC := UpdateMapInNPC{NPCIndex: 10001, NPCVNUM: 2001, PosX: 33000, PosZ: 33100, Vital: 100}
	if got, want := len(mapInNPC.Frame().Payload), 52; got != want {
		t.Fatalf("UpdateMapInNPC payload size = %d, want %d", got, want)
	}

	mapInItem := UpdateMapInItem{Info: ItemInfo{ItemIndex: 20001, ItemVNUM: 5001}, PosX: 32950, PosZ: 32950}
	if got, want := len(mapInItem.Frame().Payload), 49; got != want {
		t.Fatalf("UpdateMapInItem payload size = %d, want %d", got, want)
	}

	walk := UpdateCharWalk{CharType: 0, TargetIndex: 1001, PosX: 10, PosZ: 20, Run: true}
	if got, want := len(walk.Frame().Payload), 14; got != want {
		t.Fatalf("UpdateCharWalk payload size = %d, want %d", got, want)
	}

	place := UpdateCharPlace{CharType: 0, TargetIndex: 1001, PosX: 10, PosZ: 20, Direction: 1.5, RemainFrame: 0, Run: true}
	if got, want := len(place.Frame().Payload), 22; got != want {
		t.Fatalf("UpdateCharPlace payload size = %d, want %d", got, want)
	}

	stop := UpdateCharStop{CharType: 0, TargetIndex: 1001, PosX: 10, PosZ: 20, Direction: 1.5}
	if got, want := len(stop.Frame().Payload), 17; got != want {
		t.Fatalf("UpdateCharStop payload size = %d, want %d", got, want)
	}

	mapOut := UpdateMapOut{CharType: 0, TargetID: 1001}
	if got, want := len(mapOut.Frame().Payload), 5; got != want {
		t.Fatalf("UpdateMapOut payload size = %d, want %d", got, want)
	}

	start := ResponseGameStart{Result: 0}
	if got, want := len(start.Frame().Payload), 4; got != want {
		t.Fatalf("ResponseGameStart payload size = %d, want %d", got, want)
	}

	ready := ResponseGamePlayReady{Result: 0}
	if got, want := len(ready.Frame().Payload), 4; got != want {
		t.Fatalf("ResponseGamePlayReady payload size = %d, want %d", got, want)
	}

	pickResponse := ResponseItemPick{Info: ItemInfo{ItemIndex: 30001, ItemVNUM: 5001}}
	if got, want := len(pickResponse.Frame().Payload), 48; got != want {
		t.Fatalf("ResponseItemPick payload size = %d, want %d", got, want)
	}

	itemDrop := UpdateItemDrop{PosX: 10, PosZ: 20, Info: ItemInfo{ItemIndex: 30001, ItemVNUM: 5001}}
	if got, want := len(itemDrop.Frame().Payload), 48; got != want {
		t.Fatalf("UpdateItemDrop payload size = %d, want %d", got, want)
	}

	itemPick := UpdateItemPick{CharacterID: 1001}
	if got, want := len(itemPick.Frame().Payload), 4; got != want {
		t.Fatalf("UpdateItemPick payload size = %d, want %d", got, want)
	}

	charWear := UpdateCharWear{CharacterID: 1001, WearWhere: 2, ItemVNUM: 5001}
	if got, want := len(charWear.Frame().Payload), 16; got != want {
		t.Fatalf("UpdateCharWear payload size = %d, want %d", got, want)
	}
}
