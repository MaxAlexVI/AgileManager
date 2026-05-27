package analytics

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

func (s *Service) TeamLoad(actor shared.User) ([]shared.TeamMemberLoad, error) {
	if !shared.HasPermission(actor, shared.PermViewAnalytics) && !shared.HasPermission(actor, shared.PermManageSprints) {
		return nil, errors.New("у пользователя нет права смотреть аналитику")
	}
	return s.store.State().Analytics.TeamLoad, nil
}
