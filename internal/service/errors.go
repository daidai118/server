package service

import "errors"

var (
	ErrInvalidCredentials   = errors.New("service: invalid credentials")
	ErrAccountAlreadyExists = errors.New("service: account already exists")
	ErrCharacterNameTaken   = errors.New("service: character name already taken")
	ErrCharacterSlotTaken   = errors.New("service: character slot already taken")
	ErrCharacterLimit       = errors.New("service: character limit reached")
	ErrCharacterNotFound    = errors.New("service: character not found")
	ErrZoneHandoffMissing   = errors.New("service: zone handoff missing")
	ErrInventoryFull        = errors.New("service: inventory full")
	ErrInventorySlotTaken   = errors.New("service: inventory slot already taken")
	ErrItemNotFound         = errors.New("service: item not found")
)
