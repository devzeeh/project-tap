package auth

import (
	"context"
	"errors"
	"fmt"
	"project-tap/internal/pkg/storage"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

var (
	otpStore      = make(map[string]string)
	otpStoreMutex sync.RWMutex
)

type Service struct {
	repo    *Repository
	storage storage.Service
}

func NewService(repo *Repository, storage storage.Service) *Service {
	return &Service{
		repo:    repo,
		storage: storage,
	}
}

var ErrInvalidCredentials = errors.New("invalid credentials")

// LOGIN

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	user, err := s.repo.FindUserByUsername(ctx, req.Username)
	if err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}

	access, refresh, err := GenerateTokens(user.UserID, user.Role)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("generate tokens: %w", err)
	}

	result := LoginResponse{
		ID:          user.UserID,
		Username:    user.Username,
		Role:        user.Role,
		RedirectURL: redirectFor(user.Role, user.Username),
	}
	result.Tokens.Access = access
	result.Tokens.Refresh = refresh
	return result, nil
}

// redirectFor determines the appropriate post-login dashboard URL based on the user's role.
func redirectFor(role, username string) string {
	switch role {
	case "super_admin":
		return "/admin/" + username
	case "merchant_admin", "merchant_staff":
		return "/merchant/" + username + "/dashboard"
	default:
		return "/u/" + username + "/dashboard"
	}
}

