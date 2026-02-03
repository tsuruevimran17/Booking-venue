package service

import (
	"errors"
	"log/slog"
	"testing"

	"user-service/internal/models"
	"user-service/internal/repository"
)

type fakeUserRepo struct {
	usersByEmail map[string]*models.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{usersByEmail: make(map[string]*models.User)}
}

func (r *fakeUserRepo) Create(user *models.User) error {
	if _, exists := r.usersByEmail[user.Email]; exists {
		return errors.New("user exists")
	}
	user.ID = uint(len(r.usersByEmail) + 1)
	r.usersByEmail[user.Email] = user
	return nil
}

func (r *fakeUserRepo) GetByID(id uint) (*models.User, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeUserRepo) GetByEmail(email string) (*models.User, error) {
	user, ok := r.usersByEmail[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return user, nil
}

func (r *fakeUserRepo) Delete(id uint) error {
	return errors.New("not implemented")
}

func (r *fakeUserRepo) Update(user *models.User) error {
	return errors.New("not implemented")
}

func (r *fakeUserRepo) UpdateRole(userID uint, role models.Role) error {
	return errors.New("not implemented")
}

var _ repository.UserRepository = (*fakeUserRepo)(nil)

func TestAuthService_RegisterAndLogin(t *testing.T) {
	repo := newFakeUserRepo()
	logger := slog.Default()
	authService := NewAuthService(logger, "secret", repo)

	token, err := authService.RegisterUser(models.RegisterRequest{
		FullName: "Test User",
		Email:    "test@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected token on register")
	}

	loginToken, err := authService.LoginUser(models.LoginRequest{
		Email:    "test@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginToken == "" {
		t.Fatal("expected token on login")
	}
}
