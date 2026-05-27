package tasks

import (
	"testing"

	"agile-manager/internal/shared"
)

func TestWorkerCannotUpdateOwnTaskStatus(t *testing.T) {
	store := shared.NewMemoryStore(shared.SeedData())
	service := New(store)
	worker, ok := shared.FindUser(store.State().Users, "USR-002")
	if !ok {
		t.Fatal("seed worker USR-002 not found")
	}
	task, err := store.CreateTask(shared.TaskInput{Title: "Worker status update", Status: shared.StatusInProgress, AssigneeID: worker.ID})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	nextStatus := shared.StatusReview
	if _, err := service.Update(worker, task.ID, shared.TaskPatch{Status: &nextStatus}); err == nil {
		t.Fatal("worker updated task status, want permission error")
	}
}

func TestWorkerCanCompleteOwnTaskWithoutComment(t *testing.T) {
	store := shared.NewMemoryStore(shared.SeedData())
	service := New(store)
	worker, ok := shared.FindUser(store.State().Users, "USR-002")
	if !ok {
		t.Fatal("seed worker USR-002 not found")
	}
	task, err := store.CreateTask(shared.TaskInput{Title: "Worker completion", Status: shared.StatusInProgress, AssigneeID: worker.ID})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	updated, err := service.CompleteWork(worker, task.ID, shared.CompleteTaskInput{})
	if err != nil {
		t.Fatalf("CompleteWork: %v", err)
	}
	if !updated.WorkDone {
		t.Fatal("WorkDone is false after worker completion mark")
	}
	if len(updated.Comments) != 0 {
		t.Fatalf("comments = %d, want 0 for empty completion comment", len(updated.Comments))
	}
}
