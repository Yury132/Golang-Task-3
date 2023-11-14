package service

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/Yury132/Golang-Task-3/internal/models"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Рандомная строка
const oauthStateString = "pseudo-random"

type Service interface {
	GetUserInfo(state string, code string) ([]byte, error)
	GetUsersList(ctx context.Context) ([]models.User, error)
	HandleUser(ctx context.Context, name string, email string) error
}

type GoogleAPI interface {
	GetUserInfo(token *oauth2.Token) ([]byte, error)
}

type Storage interface {
	// Все пользователи в БД
	GetUsers(ctx context.Context) ([]models.User, error)
	// Проверка на существование пользователя
	CheckUser(ctx context.Context, email string) (bool, error)
	// Создание нового пользователя
	CreateUser(ctx context.Context, name string, email string) error
}

type service struct {
	logger      zerolog.Logger
	oauthConfig *oauth2.Config
	googleAPI   GoogleAPI
	storage     Storage
}

// Получаем данные о пользователи из Гугл
func (s *service) GetUserInfo(state string, code string) ([]byte, error) {
	if state != oauthStateString {
		return nil, fmt.Errorf("invalid oauth state")
	}

	token, err := s.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}

	contents, err := s.googleAPI.GetUserInfo(token)
	if err != nil {
		return nil, err
	}

	return contents, nil
}

// Все пользователи в БД
func (s *service) GetUsersList(ctx context.Context) ([]models.User, error) {
	users, err := s.storage.GetUsers(ctx)
	if err != nil {
		return nil, err
	}

	return users, nil
}

// Проверка существования пользователя в БД и его создание при необходимости
func (s *service) HandleUser(ctx context.Context, name string, email string) error {
	ok, err := s.checkUser(ctx, email)
	if err != nil {
		return errors.Wrap(err, "failed to check user")
	}

	if !ok {
		if err = s.createUser(ctx, name, email); err != nil {
			return errors.Wrap(err, "failed to create user")
		}
	}

	return nil
}

// Проверка на существование пользователя
func (s *service) checkUser(ctx context.Context, email string) (bool, error) {
	check, err := s.storage.CheckUser(ctx, email)
	if err != nil {
		return false, err
	}

	return check, nil
}

// Создание нового пользователя
func (s *service) createUser(ctx context.Context, name string, email string) error {
	err := s.storage.CreateUser(ctx, name, email)
	if err != nil {
		return err
	}

	return nil
}

func New(logger zerolog.Logger, oauthConfig *oauth2.Config, googleAPI GoogleAPI, storage Storage) Service {
	return &service{
		logger:      logger,
		oauthConfig: oauthConfig,
		googleAPI:   googleAPI,
		storage:     storage,
	}
}
