package authselect

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"laghaim-go/internal/protocol"
	"laghaim-go/internal/service"
)

type Config struct {
	ZoneHost string
	ZonePort int
}

type Server struct {
	listener net.Listener
	codec    protocol.SeedCodec
	auth     service.AuthService
	chars    service.CharacterService
	config   Config

	closeOnce sync.Once
	closed    chan struct{}
}

func NewServer(listener net.Listener, codec protocol.SeedCodec, auth service.AuthService, chars service.CharacterService, config Config) *Server {
	return &Server{
		listener: listener,
		codec:    codec,
		auth:     auth,
		chars:    chars,
		config:   config,
		closed:   make(chan struct{}),
	}
}

func (s *Server) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return nil
			default:
			}
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closed)
		err = s.listener.Close()
	})
	return err
}

type connState struct {
	remoteIP string
	account  *service.LoginResult
	pending  string
	username string
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	state := connState{remoteIP: remoteIP(conn)}
	ctx := context.Background()

	for {
		frame, err := protocol.ReadFrame(conn, s.codec)
		if err != nil {
			return
		}
		if !protocol.IsTextCommand(frame) {
			continue
		}

		line := strings.TrimSpace(string(frame.Payload))
		if line == "" {
			continue
		}
		if _, err := strconv.Atoi(line); err == nil {
			continue // version frame
		}

		if state.account == nil {
			if err := s.handleAuthLine(ctx, conn, &state, line); err != nil {
				return
			}
			continue
		}

		if err := s.handleCommand(ctx, conn, &state, line); err != nil {
			return
		}
	}
}

func (s *Server) handleAuthLine(ctx context.Context, conn net.Conn, state *connState, line string) error {
	switch state.pending {
	case "":
		if line == "login" || line == "register" {
			state.pending = line
			return nil
		}
		return s.writeFail(conn, "unexpected_command")
	case "login", "register":
		state.username = line
		state.pending = state.pending + ":password"
		return nil
	case "login:password":
		password := parsePassword(line)
		result, err := s.auth.Login(ctx, state.username, password, state.remoteIP)
		if err != nil {
			return s.writeFail(conn, authErrorCode(err))
		}
		state.account = &result
		state.pending = ""
		state.username = ""
		return s.writeCharacterList(ctx, conn, result.AccountID)
	case "register:password":
		password := parsePassword(line)
		result, err := s.auth.Register(ctx, state.username, password, state.remoteIP)
		if err != nil {
			return s.writeFail(conn, authErrorCode(err))
		}
		state.account = &result
		state.pending = ""
		state.username = ""
		return s.writeCharacterList(ctx, conn, result.AccountID)
	default:
		return s.writeFail(conn, "invalid_auth_state")
	}
}

func (s *Server) handleCommand(ctx context.Context, conn net.Conn, state *connState, line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}

	switch fields[0] {
	case "char_exist":
		if len(fields) < 2 {
			return s.writeFail(conn, "bad_char_exist")
		}
		available, err := s.chars.IsNameAvailable(ctx, fields[1])
		if err != nil {
			return s.writeFail(conn, "internal_error")
		}
		if available {
			return protocol.WriteTextCommand(conn, s.codec, "success\n")
		}
		return protocol.WriteTextCommand(conn, s.codec, "fail\n")
	case "char_new":
		if len(fields) < 11 {
			return s.writeFail(conn, "bad_char_new")
		}
		slotIndex, err := parseUint8(fields[1])
		if err != nil {
			return s.writeFail(conn, "bad_slot")
		}
		race, err := parseUint8(fields[3])
		if err != nil {
			return s.writeFail(conn, "bad_race")
		}
		sex, err := parseUint8(fields[4])
		if err != nil {
			return s.writeFail(conn, "bad_sex")
		}
		hair, err := parseUint8(fields[5])
		if err != nil {
			return s.writeFail(conn, "bad_hair")
		}
		_, err = s.chars.CreateCharacter(ctx, service.CreateCharacterRequest{
			AccountID: state.account.AccountID,
			SlotIndex: slotIndex,
			Name:      fields[2],
			Race:      race,
			Sex:       sex,
			Hair:      hair,
		})
		if err != nil {
			return s.writeFail(conn, charErrorCode(err))
		}
		return protocol.WriteTextCommand(conn, s.codec, "success\n")
	case "char_del":
		if len(fields) < 3 {
			return s.writeFail(conn, "bad_char_del")
		}
		characterID, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			return s.writeFail(conn, "bad_char_id")
		}
		if err := s.chars.DeleteCharacter(ctx, state.account.AccountID, characterID); err != nil {
			return s.writeFail(conn, charErrorCode(err))
		}
		return protocol.WriteTextCommand(conn, s.codec, "success\n")
	case "start":
		if len(fields) < 2 {
			return s.writeFail(conn, "bad_start")
		}
		slotIndex, err := parseUint8(fields[1])
		if err != nil {
			return s.writeFail(conn, "bad_slot")
		}
		characters, err := s.chars.ListCharacters(ctx, state.account.AccountID)
		if err != nil {
			return s.writeFail(conn, "internal_error")
		}
		selected, ok := characterBySlot(characters, slotIndex)
		if !ok {
			return s.writeFail(conn, "char_not_found")
		}
		if _, err := s.chars.SelectCharacter(ctx, state.account.AccountID, selected.CharacterID); err != nil {
			return s.writeFail(conn, charErrorCode(err))
		}
		return protocol.WriteTextCommand(conn, s.codec, fmt.Sprintf("go_world %s %d %d %d\n", s.config.ZoneHost, s.config.ZonePort, selected.MapID, selected.ZoneID))
	case "chars":
		return s.writeCharacterList(ctx, conn, state.account.AccountID)
	default:
		return s.writeFail(conn, "unknown_command")
	}
}

func (s *Server) writeCharacterList(ctx context.Context, conn net.Conn, accountID uint64) error {
	characters, err := s.chars.ListCharacters(ctx, accountID)
	if err != nil {
		return err
	}
	if err := protocol.WriteTextCommand(conn, s.codec, "chars_start\n"); err != nil {
		return err
	}
	for _, character := range characters {
		line := fmt.Sprintf(
			"chars_exist %d %d %s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d\n",
			character.SlotIndex,
			character.CharacterID,
			character.Name,
			character.Race,
			character.Sex,
			character.Hair,
			character.Level,
			character.Vital,
			character.MaxVital,
			character.Mana,
			character.MaxMana,
			character.Stamina,
			character.MaxStamina,
			character.EPower,
			character.MaxEPower,
			character.Strength,
			character.Intelligence,
			character.Dexterity,
			character.Constitution,
			character.Charisma,
			character.BlockedTime,
			character.GuildIndex,
			character.Wearings[0],
			character.Wearings[1],
			character.Wearings[2],
			character.Wearings[3],
			character.Wearings[4],
			character.Wearings[5],
			character.Wearings[6],
			character.Wearings[7],
			boolToInt(character.IsGuildMaster),
			boolToInt(character.IsSupport),
		)
		if err := protocol.WriteTextCommand(conn, s.codec, line); err != nil {
			return err
		}
	}
	return protocol.WriteTextCommand(conn, s.codec, "chars_end 0 0\n")
}

func (s *Server) writeFail(conn net.Conn, reason string) error {
	return protocol.WriteTextCommand(conn, s.codec, fmt.Sprintf("fail %s\n", reason))
}

func authErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return "invalid_credentials"
	case errors.Is(err, service.ErrAccountAlreadyExists):
		return "account_exists"
	default:
		return "internal_error"
	}
}

func charErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrCharacterNameTaken):
		return "name_taken"
	case errors.Is(err, service.ErrCharacterSlotTaken):
		return "slot_taken"
	case errors.Is(err, service.ErrCharacterLimit):
		return "char_limit"
	case errors.Is(err, service.ErrCharacterNotFound):
		return "char_not_found"
	default:
		return "internal_error"
	}
}

func parsePassword(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func parseUint8(raw string) (uint8, error) {
	value, err := strconv.ParseUint(raw, 10, 8)
	return uint8(value), err
}

func characterBySlot(characters []service.CharacterSummary, slotIndex uint8) (service.CharacterSummary, bool) {
	for _, character := range characters {
		if character.SlotIndex == slotIndex {
			return character, true
		}
	}
	return service.CharacterSummary{}, false
}

func remoteIP(conn net.Conn) string {
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return conn.RemoteAddr().String()
	}
	return host
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
