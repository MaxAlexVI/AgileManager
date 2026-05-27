package users

import (
	"errors"

	"agile-manager/internal/shared"
)

type Service struct {
	store *shared.Store
}

func New(store *shared.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(actor shared.User, input shared.UserInput) (shared.User, error) {
	if !shared.HasPermission(actor, shared.PermManageUsers) {
		return shared.User{}, errors.New("у пользователя нет права управлять пользователями")
	}
	return s.store.CreateUser(input)
}

func (s *Service) Update(actor shared.User, id string, input shared.UserInput) (shared.User, error) {
	if !shared.HasPermission(actor, shared.PermManageUsers) {
		return shared.User{}, errors.New("у пользователя нет права управлять пользователями")
	}
	return s.store.UpdateUser(id, input)
}
