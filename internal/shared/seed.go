package shared

import "time"

func seedData() StoreData {
	now := time.Now().UTC()
	doneAt := now.Add(-18 * time.Hour)

	users := []User{
		{ID: "USR-001", Login: "manager", Name: "Виктор Пелевин", RoleID: RoleManager, Role: "Руководитель", Email: "viktor@example.local", Password: "manager2026"},
		{ID: "USR-002", Login: "worker1", Name: "Мишель Фуко", RoleID: RoleDeveloper, Role: "Пользователь", Email: "michel@example.local", Password: "user2026"},
		{ID: "USR-003", Login: "worker2", Name: "Роберт Смит", RoleID: RoleDeveloper, Role: "Пользователь", Email: "robert@example.local", Password: "user2026"},
	}

	sprints := []Sprint{
		{
			ID:        "SPR-001",
			Name:      "Sprint 01",
			Goal:      "Запустить базовый Scrumban-процесс и прозрачную доску задач.",
			StartDate: now.AddDate(0, 0, -5).Format("2006-01-02"),
			EndDate:   now.AddDate(0, 0, 9).Format("2006-01-02"),
			Status:    "active",
		},
		{
			ID:            "SPR-002",
			Name:          "Discovery",
			Goal:          "Собрать требования и подготовить структуру модулей.",
			StartDate:     now.AddDate(0, 0, -20).Format("2006-01-02"),
			EndDate:       now.AddDate(0, 0, -7).Format("2006-01-02"),
			Status:        "closed",
			Retrospective: "Команде помогли короткие ежедневные синки и явные WIP-лимиты.",
		},
	}

	tasks := []Task{
		{
			ID:          "TASK-001",
			Title:       "Описать роли и права доступа",
			Description: "Подготовить матрицу прав для администратора, руководителя и пользователя.",
			Status:      StatusBacklog,
			Priority:    "high",
			AssigneeID:  "USR-001",
			ReporterID:  "USR-001",
			DueDate:     now.AddDate(0, 0, 4).Format("2006-01-02"),
			StoryPoints: 3,
			SprintID:    "SPR-001",
			CreatedAt:   now.AddDate(0, 0, -3),
			UpdatedAt:   now.AddDate(0, 0, -2),
			Comments: []Comment{
				{ID: "CMT-001", TaskID: "TASK-001", AuthorID: "USR-001", Text: "Нужно отдельно отметить разделение прав руководителя и пользователя.", CreatedAt: now.AddDate(0, 0, -2)},
			},
		},
		{
			ID:          "TASK-002",
			Title:       "Собрать API для карточек задач",
			Description: "Создание, обновление статуса, назначение исполнителя, дедлайны и приоритеты.",
			Status:      StatusInProgress,
			Priority:    "high",
			AssigneeID:  "USR-002",
			ReporterID:  "USR-001",
			DueDate:     now.AddDate(0, 0, 2).Format("2006-01-02"),
			StoryPoints: 8,
			SprintID:    "SPR-001",
			CreatedAt:   now.AddDate(0, 0, -4),
			UpdatedAt:   now.Add(-3 * time.Hour),
		},
		{
			ID:          "TASK-003",
			Title:       "Проверить сценарий Kanban drag-and-drop",
			Description: "Проверить перемещение задач по колонкам Backlog, To Do, In Progress, Review и Done.",
			Status:      StatusReview,
			Priority:    "medium",
			AssigneeID:  "USR-003",
			ReporterID:  "USR-001",
			DueDate:     now.AddDate(0, 0, 5).Format("2006-01-02"),
			StoryPoints: 5,
			SprintID:    "SPR-001",
			CreatedAt:   now.AddDate(0, 0, -5),
			UpdatedAt:   now.Add(-6 * time.Hour),
		},
		{
			ID:          "TASK-004",
			Title:       "Сформировать первый отчет по загрузке",
			Description: "Показать активные задачи, завершенные задачи, story points и ближайшие дедлайны.",
			Status:      StatusDone,
			Priority:    "medium",
			AssigneeID:  "USR-001",
			ReporterID:  "USR-001",
			DueDate:     now.AddDate(0, 0, -1).Format("2006-01-02"),
			StoryPoints: 3,
			SprintID:    "SPR-002",
			CreatedAt:   now.AddDate(0, 0, -10),
			UpdatedAt:   doneAt,
			CompletedAt: doneAt,
		},
	}

	notifications := []Notification{
		{ID: "NTF-001", Type: "task.status", Message: "Задача Проверить сценарий Kanban drag-and-drop перемещена в Review", TaskID: "TASK-003", UserID: "USR-003", CreatedAt: now.Add(-6 * time.Hour)},
		{ID: "NTF-002", Type: "report.daily", Message: "Ежедневный отчет по загрузке команды готов", CreatedAt: now.Add(-24 * time.Hour)},
	}

	return StoreData{
		Users:         users,
		Tasks:         tasks,
		Sprints:       sprints,
		Notifications: notifications,
		Next: map[string]int{
			"task":         4,
			"user":         3,
			"comment":      1,
			"sprint":       2,
			"notification": 2,
		},
	}
}

func SeedData() StoreData {
	return seedData()
}
