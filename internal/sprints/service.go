package sprints

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

func (s *Service) Create(actor shared.User, input shared.SprintInput) (shared.Sprint, error) {
	if !shared.HasPermission(actor, shared.PermManageSprints) {
		return shared.Sprint{}, errors.New("у пользователя нет права создавать спринты")
	}
	return s.store.CreateSprint(input)
}

func (s *Service) Update(actor shared.User, id string, input shared.SprintInput) (shared.Sprint, error) {
	if !shared.HasPermission(actor, shared.PermManageSprints) {
		return shared.Sprint{}, errors.New("у пользователя нет права изменять спринты")
	}
	return s.store.UpdateSprint(id, input)
}
