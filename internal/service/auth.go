package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"laghaim-go/internal/repo"
	"laghaim-go/internal/session"
)

type AuthConfig struct {
	GMSTicketTTL time.Duration
}

type authService struct {
	accounts repo.AccountRepository
	sessions *session.Manager
	hasher   PasswordHasher
	config   AuthConfig
}

func NewAuthService(accounts repo.AccountRepository, sessions *session.Manager, hasher PasswordHasher, config AuthConfig) AuthService {
	if config.GMSTicketTTL <= 0 {
		config.GMSTicketTTL = 2 * time.Minute
	}
	return &authService{
		accounts: accounts,
		sessions: sessions,
		hasher:   hasher,
		config:   config,
	}
}

func (s *authService) Register(ctx context.Context, username, password, remoteIP string) (LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return LoginResult{}, err
	}

	account, err := s.accounts.CreateAccount(ctx, repo.CreateAccountParams{
		Username:     username,
		PasswordHash: passwordHash,
		PasswordAlgo: "argon2id",
	})
	if err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return LoginResult{}, ErrAccountAlreadyExists
		}
		return LoginResult{}, err
	}

	return s.loginAccount(ctx, account, remoteIP)
}

func (s *authService) Login(ctx context.Context, username, password, remoteIP string) (LoginResult, error) {
	account, err := s.accounts.GetAccountByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	if account.Status != "active" {
		return LoginResult{}, ErrInvalidCredentials
	}
	if !s.hasher.Verify(password, account.PasswordHash) {
		return LoginResult{}, ErrInvalidCredentials
	}

	return s.loginAccount(ctx, account, remoteIP)
}

func (s *authService) loginAccount(ctx context.Context, account repo.Account, remoteIP string) (LoginResult, error) {
	sessionState := s.sessions.StartAccountLogin(int64(account.ID))
	gmsTicket, err := s.sessions.IssueGMSTicket(int64(account.ID), s.config.GMSTicketTTL)
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.accounts.UpdateLoginMetadata(ctx, repo.UpdateLoginMetadataParams{
		AccountID:   account.ID,
		LastLoginAt: time.Now().UTC(),
		LastLoginIP: remoteIP,
	}); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		AccountID: account.ID,
		SessionID: sessionState.SessionID,
		GMSTicket: gmsTicket.ID,
	}, nil
}
