package protocol

import (
	"bytes"
	"encoding/binary"
	"math"
)

type RequestCharWalk struct {
	PosX float32
	PosZ float32
	Run  bool
}

func DecodeRequestCharWalk(payload []byte) (RequestCharWalk, error) {
	if len(payload) < 9 {
		return RequestCharWalk{}, ErrFrameTooShort
	}
	return RequestCharWalk{
		PosX: math.Float32frombits(binary.LittleEndian.Uint32(payload[0:4])),
		PosZ: math.Float32frombits(binary.LittleEndian.Uint32(payload[4:8])),
		Run:  payload[8] != 0,
	}, nil
}

type RequestCharPlace struct {
	CharType    byte
	PosX        float32
	PosZ        float32
	Direction   float32
	RemainFrame int32
	Run         bool
}

func DecodeRequestCharPlace(payload []byte) (RequestCharPlace, error) {
	if len(payload) < 18 {
		return RequestCharPlace{}, ErrFrameTooShort
	}
	return RequestCharPlace{
		CharType:    payload[0],
		PosX:        math.Float32frombits(binary.LittleEndian.Uint32(payload[1:5])),
		PosZ:        math.Float32frombits(binary.LittleEndian.Uint32(payload[5:9])),
		Direction:   math.Float32frombits(binary.LittleEndian.Uint32(payload[9:13])),
		RemainFrame: int32(binary.LittleEndian.Uint32(payload[13:17])),
		Run:         payload[17] != 0,
	}, nil
}

type RequestCharStop struct {
	CharType  byte
	PosX      float32
	PosZ      float32
	Direction float32
}

func DecodeRequestCharStop(payload []byte) (RequestCharStop, error) {
	if len(payload) < 13 {
		return RequestCharStop{}, ErrFrameTooShort
	}
	return RequestCharStop{
		CharType:  payload[0],
		PosX:      math.Float32frombits(binary.LittleEndian.Uint32(payload[1:5])),
		PosZ:      math.Float32frombits(binary.LittleEndian.Uint32(payload[5:9])),
		Direction: math.Float32frombits(binary.LittleEndian.Uint32(payload[9:13])),
	}, nil
}

type RequestWear struct {
	WearWhere int32
}

func DecodeRequestWear(payload []byte) (RequestWear, error) {
	if len(payload) < 4 {
		return RequestWear{}, ErrFrameTooShort
	}
	return RequestWear{
		WearWhere: int32(binary.LittleEndian.Uint32(payload[0:4])),
	}, nil
}

type RequestItemDrop struct {
	ItemIndex int32
}

func DecodeRequestItemDrop(payload []byte) (RequestItemDrop, error) {
	if len(payload) < 4 {
		return RequestItemDrop{}, ErrFrameTooShort
	}
	return RequestItemDrop{ItemIndex: int32(binary.LittleEndian.Uint32(payload[0:4]))}, nil
}

type RequestItemPick struct {
	ItemIndex int32
}

func DecodeRequestItemPick(payload []byte) (RequestItemPick, error) {
	if len(payload) < 4 {
		return RequestItemPick{}, ErrFrameTooShort
	}
	return RequestItemPick{ItemIndex: int32(binary.LittleEndian.Uint32(payload[0:4]))}, nil
}

type ResponseGameStart struct {
	Result int32
}

func (r ResponseGameStart) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: ResGameStart, Type: PacketTypeResponse},
		Payload:   marshalGameStartResponse(r),
	}
}

type ResponseGamePlayReady struct {
	Result int32
}

func (r ResponseGamePlayReady) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: ResGamePlayReady, Type: PacketTypeResponse},
		Payload:   marshalGamePlayReadyResponse(r),
	}
}

type ResponseItemPick struct {
	InventoryPack int32
	SlotX         int32
	SlotY         int32
	Info          ItemInfo
}

func (r ResponseItemPick) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: ResItemPick, Type: PacketTypeResponse},
		Payload:   marshalItemPickResponse(r),
	}
}

type UpdateCharWalk struct {
	CharType    byte
	TargetIndex int32
	PosX        float32
	PosZ        float32
	Run         bool
}

func (u UpdateCharWalk) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdCharWalk, Type: PacketTypeUpdate},
		Payload:   marshalCharWalkUpdate(u),
	}
}

type UpdateCharPlace struct {
	CharType    byte
	TargetIndex int32
	PosX        float32
	PosZ        float32
	Direction   float32
	RemainFrame int32
	Run         bool
}

func (u UpdateCharPlace) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdCharPlace, Type: PacketTypeUpdate},
		Payload:   marshalCharPlaceUpdate(u),
	}
}

type UpdateCharStop struct {
	CharType    byte
	TargetIndex int32
	PosX        float32
	PosZ        float32
	Direction   float32
}

func (u UpdateCharStop) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdCharStop, Type: PacketTypeUpdate},
		Payload:   marshalCharStopUpdate(u),
	}
}

type UpdateMapInChar struct {
	Type        byte
	CharacterID int32
	Name        string
	Race        int32
	Sex         int32
	Hair        int32
	PosX        float32
	PosZ        float32
	Direction   float32
	Wearings    [CharMapInWearCount]int32
	Vital       int32
	CombatState int32
	SkillIndex  int32
	State       int32
	SubSkill    int32
	ChaoGrade   int32
	PK          int32
	Sky         int32
	GuildName   string
	GuildGrade  string
	GuildLevel  int32
	GuildIndex  int32
	GuildType   int32
}

func (u UpdateMapInChar) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdMapInChar, Type: PacketTypeUpdate},
		Payload:   marshalMapInCharUpdate(u),
	}
}

type UpdateMapInNPC struct {
	NPCIndex     int32
	NPCVNUM      int32
	PosX         float32
	PosZ         float32
	Direction    float32
	MobEventFlag int32
	Vital        int32
	Mutant       int32
	Attribute    int32
	MobClass     int32
	State        int32
	QuestLevel   int32
	Regen        int32
}

func (u UpdateMapInNPC) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdMapInNpc, Type: PacketTypeUpdate},
		Payload:   marshalMapInNPCUpdate(u),
	}
}

type ItemInfo struct {
	ItemIndex           int32
	ItemVNUM            int32
	PlusPoint           int32
	SpecialFlag1        int32
	SpecialFlag2        int32
	UpgradeEndurance    int32
	UpgradeEnduranceMax int32
	Endurance           int32
	EnduranceMax        int32
}

type UpdateMapInItem struct {
	Info      ItemInfo
	TimedItem bool
	PosX      float32
	PosZ      float32
	Direction float32
}

type UpdateItemDrop struct {
	PosX      float32
	PosZ      float32
	Direction float32
	Info      ItemInfo
}

func (u UpdateItemDrop) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdItemDrop, Type: PacketTypeUpdate},
		Payload:   marshalItemDropUpdate(u),
	}
}

type UpdateItemPick struct {
	CharacterID int32
}

func (u UpdateItemPick) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdItemPick, Type: PacketTypeUpdate},
		Payload:   marshalItemPickUpdate(u),
	}
}

type UpdateCharWear struct {
	CharacterID   int32
	WearWhere     int32
	ItemVNUM      int32
	ItemPlusPoint int32
}

func (u UpdateCharWear) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdCharWear, Type: PacketTypeUpdate},
		Payload:   marshalCharWearUpdate(u),
	}
}

func (u UpdateMapInItem) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdMapInItem, Type: PacketTypeUpdate},
		Payload:   marshalMapInItemUpdate(u),
	}
}

type UpdateMapOut struct {
	CharType byte
	TargetID int32
}

func (u UpdateMapOut) Frame() Frame {
	return Frame{
		SubHeader: SubHeader{Index: UpdMapOut, Type: PacketTypeUpdate},
		Payload:   marshalMapOutUpdate(u),
	}
}

func marshalGameStartResponse(response ResponseGameStart) []byte {
	return marshalInt32Response(response.Result)
}

func marshalInt32Response(result int32) []byte {
	var body bytes.Buffer
	writeInt32LE(&body, result)
	return body.Bytes()
}

func marshalItemPickResponse(response ResponseItemPick) []byte {
	var body bytes.Buffer
	writeInt32LE(&body, response.InventoryPack)
	writeInt32LE(&body, response.SlotX)
	writeInt32LE(&body, response.SlotY)
	writeItemInfo(&body, response.Info)
	return body.Bytes()
}

func marshalGamePlayReadyResponse(response ResponseGamePlayReady) []byte {
	var body bytes.Buffer
	writeInt32LE(&body, response.Result)
	return body.Bytes()
}

func marshalCharWalkUpdate(update UpdateCharWalk) []byte {
	var body bytes.Buffer
	body.WriteByte(update.CharType)
	writeInt32LE(&body, update.TargetIndex)
	writeFloat32LE(&body, update.PosX)
	writeFloat32LE(&body, update.PosZ)
	writeBoolByte(&body, update.Run)
	return body.Bytes()
}

func marshalCharPlaceUpdate(update UpdateCharPlace) []byte {
	var body bytes.Buffer
	body.WriteByte(update.CharType)
	writeInt32LE(&body, update.TargetIndex)
	writeFloat32LE(&body, update.PosX)
	writeFloat32LE(&body, update.PosZ)
	writeFloat32LE(&body, update.Direction)
	writeInt32LE(&body, update.RemainFrame)
	writeBoolByte(&body, update.Run)
	return body.Bytes()
}

func marshalCharStopUpdate(update UpdateCharStop) []byte {
	var body bytes.Buffer
	body.WriteByte(update.CharType)
	writeInt32LE(&body, update.TargetIndex)
	writeFloat32LE(&body, update.PosX)
	writeFloat32LE(&body, update.PosZ)
	writeFloat32LE(&body, update.Direction)
	return body.Bytes()
}

func marshalMapInCharUpdate(update UpdateMapInChar) []byte {
	var body bytes.Buffer
	body.WriteByte(update.Type)
	writeInt32LE(&body, update.CharacterID)
	writeFixedString(&body, update.Name, UserIDLength)
	writeInt32LE(&body, update.Race)
	writeInt32LE(&body, update.Sex)
	writeInt32LE(&body, update.Hair)
	writeFloat32LE(&body, update.PosX)
	writeFloat32LE(&body, update.PosZ)
	writeFloat32LE(&body, update.Direction)
	for _, wearing := range update.Wearings {
		writeInt32LE(&body, wearing)
	}
	writeInt32LE(&body, update.Vital)
	writeInt32LE(&body, update.CombatState)
	writeInt32LE(&body, update.SkillIndex)
	writeInt32LE(&body, update.State)
	writeInt32LE(&body, update.SubSkill)
	writeInt32LE(&body, update.ChaoGrade)
	writeInt32LE(&body, update.PK)
	writeInt32LE(&body, update.Sky)
	writeFixedString(&body, update.GuildName, GuildNameLength)
	writeFixedString(&body, update.GuildGrade, GuildGradeLength)
	writeInt32LE(&body, update.GuildLevel)
	writeInt32LE(&body, update.GuildIndex)
	writeInt32LE(&body, update.GuildType)
	return body.Bytes()
}

func marshalMapInNPCUpdate(update UpdateMapInNPC) []byte {
	var body bytes.Buffer
	writeInt32LE(&body, update.NPCIndex)
	writeInt32LE(&body, update.NPCVNUM)
	writeFloat32LE(&body, update.PosX)
	writeFloat32LE(&body, update.PosZ)
	writeFloat32LE(&body, update.Direction)
	writeInt32LE(&body, update.MobEventFlag)
	writeInt32LE(&body, update.Vital)
	writeInt32LE(&body, update.Mutant)
	writeInt32LE(&body, update.Attribute)
	writeInt32LE(&body, update.MobClass)
	writeInt32LE(&body, update.State)
	writeInt32LE(&body, update.QuestLevel)
	writeInt32LE(&body, update.Regen)
	return body.Bytes()
}

func marshalMapInItemUpdate(update UpdateMapInItem) []byte {
	var body bytes.Buffer
	writeItemInfo(&body, update.Info)
	writeBoolByte(&body, update.TimedItem)
	writeFloat32LE(&body, update.PosX)
	writeFloat32LE(&body, update.PosZ)
	writeFloat32LE(&body, update.Direction)
	return body.Bytes()
}

func marshalItemDropUpdate(update UpdateItemDrop) []byte {
	var body bytes.Buffer
	writeFloat32LE(&body, update.PosX)
	writeFloat32LE(&body, update.PosZ)
	writeFloat32LE(&body, update.Direction)
	writeItemInfo(&body, update.Info)
	return body.Bytes()
}

func marshalMapOutUpdate(update UpdateMapOut) []byte {
	var body bytes.Buffer
	body.WriteByte(update.CharType)
	writeInt32LE(&body, update.TargetID)
	return body.Bytes()
}

func marshalItemPickUpdate(update UpdateItemPick) []byte {
	var body bytes.Buffer
	writeInt32LE(&body, update.CharacterID)
	return body.Bytes()
}

func marshalCharWearUpdate(update UpdateCharWear) []byte {
	var body bytes.Buffer
	writeInt32LE(&body, update.CharacterID)
	writeInt32LE(&body, update.WearWhere)
	writeInt32LE(&body, update.ItemVNUM)
	writeInt32LE(&body, update.ItemPlusPoint)
	return body.Bytes()
}

func writeItemInfo(buf *bytes.Buffer, info ItemInfo) {
	writeInt32LE(buf, info.ItemIndex)
	writeInt32LE(buf, info.ItemVNUM)
	writeInt32LE(buf, info.PlusPoint)
	writeInt32LE(buf, info.SpecialFlag1)
	writeInt32LE(buf, info.SpecialFlag2)
	writeInt32LE(buf, info.UpgradeEndurance)
	writeInt32LE(buf, info.UpgradeEnduranceMax)
	writeInt32LE(buf, info.Endurance)
	writeInt32LE(buf, info.EnduranceMax)
}

func writeInt32LE(buf *bytes.Buffer, value int32) {
	_ = binary.Write(buf, binary.LittleEndian, value)
}

func writeFloat32LE(buf *bytes.Buffer, value float32) {
	_ = binary.Write(buf, binary.LittleEndian, value)
}

func writeBoolByte(buf *bytes.Buffer, value bool) {
	if value {
		buf.WriteByte(1)
		return
	}
	buf.WriteByte(0)
}

func writeFixedString(buf *bytes.Buffer, value string, size int) {
	data := make([]byte, size)
	copy(data, []byte(value))
	buf.Write(data)
}
