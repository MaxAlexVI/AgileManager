package tasks

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

func (s *Service) Create(actor shared.User, input shared.TaskInput) (shared.Task, error) {
	if !shared.HasPermission(actor, shared.PermCreateTasks) {
		return shared.Task{}, errors.New("у пользователя нет права создавать задачи")
	}
	return s.store.CreateTask(input)
}

func (s *Service) Update(actor shared.User, id string, patch shared.TaskPatch) (shared.Task, error) {
	state := s.store.State()
	if _, ok := shared.FindTask(state.Tasks, id); !ok {
		return shared.Task{}, shared.ErrNotFound("task", id)
	}
	if shared.HasPermission(actor, shared.PermEditAnyTask) {
		return s.store.UpdateTask(id, patch)
	}
	return shared.Task{}, errors.New("у пользователя нет права изменять эту задачу")
}

func (s *Service) Comment(actor shared.User, taskID string, input shared.CommentInput) (shared.Task, error) {
	if !shared.HasPermission(actor, shared.PermCommentTasks) {
		return shared.Task{}, errors.New("у пользователя нет права комментировать задачи")
	}
	if input.AuthorID == "" {
		input.AuthorID = actor.ID
	}
	return s.store.AddComment(taskID, input)
}

func (s *Service) CompleteWork(actor shared.User, taskID string, input shared.CompleteTaskInput) (shared.Task, error) {
	state := s.store.State()
	task, ok := shared.FindTask(state.Tasks, taskID)
	if !ok {
		return shared.Task{}, shared.ErrNotFound("task", taskID)
	}
	if !shared.HasPermission(actor, shared.PermEditAnyTask) && !(shared.HasPermission(actor, shared.PermCompleteOwnTask) && task.AssigneeID == actor.ID) {
		return shared.Task{}, errors.New("у пользователя нет права отмечать выполнение этой задачи")
	}
	return s.store.CompleteTaskWork(taskID, actor.ID, input.Comment)
}
