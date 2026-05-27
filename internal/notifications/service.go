package notifications

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

func (s *Service) Dismiss(actor shared.User, id string) (shared.Notification, error) {
	if !shared.HasPermission(actor, shared.PermDismissActivity) {
		return shared.Notification{}, errors.New("у пользователя нет права закрывать уведомления")
	}
	return s.store.DismissNotification(id)
}
