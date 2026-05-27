package shared

import "testing"

func TestCreateTaskDefaults(t *testing.T) {
	store := NewMemoryStore(seedData())

	task, err := store.CreateTask(TaskInput{Title: "Новая карточка"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.Status != StatusBacklog {
		t.Fatalf("status = %q, want %q", task.Status, StatusBacklog)
	}
	if task.Priority != "medium" {
		t.Fatalf("priority = %q, want medium", task.Priority)
	}
}

func TestUpdateTaskToDoneSetsCompletedAt(t *testing.T) {
	store := NewMemoryStore(seedData())
	task, err := store.CreateTask(TaskInput{Title: "Закрыть задачу"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	done := StatusDone
	updated, err := store.UpdateTask(task.ID, TaskPatch{Status: &done})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.CompletedAt.IsZero() {
		t.Fatal("CompletedAt is zero after moving task to done")
	}
}

func TestAnalyticsTeamLoad(t *testing.T) {
	data := seedData()
	analytics := buildAnalytics(data)
	if analytics.ActiveTasks == 0 {
		t.Fatal("expected active tasks in seed analytics")
	}
	if len(analytics.TeamLoad) != len(data.Users) {
		t.Fatalf("team load items = %d, want %d", len(analytics.TeamLoad), len(data.Users))
	}
}

func TestDismissNotificationHidesRecentActivity(t *testing.T) {
	store := NewMemoryStore(seedData())

	task, err := store.CreateTask(TaskInput{Title: "Задача с уведомлением"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	state := store.State()
	if len(state.Analytics.RecentActivity) == 0 {
		t.Fatal("expected recent activity after creating task")
	}

	noteID := state.Analytics.RecentActivity[0].ID
	if _, err := store.DismissNotification(noteID); err != nil {
		t.Fatalf("DismissNotification: %v", err)
	}

	state = store.State()
	for _, note := range state.Analytics.RecentActivity {
		if note.ID == noteID {
			t.Fatalf("dismissed notification %s is still in recent activity for task %s", noteID, task.ID)
		}
	}
}

func TestCreateTaskNotifiesEveryUserIndependently(t *testing.T) {
	store := NewMemoryStore(seedData())

	task, err := store.CreateTask(TaskInput{Title: "Общее уведомление"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	state := store.State()
	notesByUser := map[string]Notification{}
	for _, note := range state.Notifications {
		if note.TaskID == task.ID && note.Type == "task.created" {
			notesByUser[note.UserID] = note
		}
	}
	for _, user := range state.Users {
		if _, ok := notesByUser[user.ID]; !ok {
			t.Fatalf("notification for user %s not found", user.ID)
		}
	}

	if _, err := store.DismissNotification(notesByUser["USR-001"].ID); err != nil {
		t.Fatalf("DismissNotification: %v", err)
	}
	managerState := store.StateForUser("USR-001")
	workerState := store.StateForUser("USR-002")
	if containsNotification(managerState.Notifications, notesByUser["USR-001"].ID) {
		t.Fatal("manager still sees dismissed notification")
	}
	if !containsNotification(workerState.Notifications, notesByUser["USR-002"].ID) {
		t.Fatal("worker notification disappeared after manager dismissed their copy")
	}
}

func TestCompleteTaskWorkAddsCommentAndKeepsStatus(t *testing.T) {
	store := NewMemoryStore(seedData())
	task, err := store.CreateTask(TaskInput{Title: "Проверить выполнение", Status: StatusInProgress, AssigneeID: "USR-002"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	updated, err := store.CompleteTaskWork(task.ID, "USR-002", "Работа готова на текущем этапе")
	if err != nil {
		t.Fatalf("CompleteTaskWork: %v", err)
	}
	if updated.Status != StatusInProgress {
		t.Fatalf("status = %q, want %q", updated.Status, StatusInProgress)
	}
	if !updated.WorkDone {
		t.Fatal("WorkDone is false after completion mark")
	}
	if len(updated.Comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(updated.Comments))
	}
}

func containsNotification(notes []Notification, id string) bool {
	for _, note := range notes {
		if note.ID == id && !note.Read {
			return true
		}
	}
	return false
}
