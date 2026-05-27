package shared

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu   sync.RWMutex
	db   *sql.DB
	data StoreData
}

func NewStore(databaseURL string) (*Store, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)

	store := &Store{db: db}
	if err := store.load(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func NewMemoryStore(data StoreData) *Store {
	store := &Store{data: data}
	store.ensureUserRolesLocked()
	store.ensureUserLoginsLocked()
	store.ensureNextCountersLocked()
	return store
}

func (s *Store) State() AppState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		logStoreError("refresh state", err)
	}

	return appStateFromData(s.data)
}

func (s *Store) StateForUser(userID string) AppState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		logStoreError("refresh user state", err)
	}

	data := s.data
	data.Notifications = notificationsForUser(s.data.Notifications, userID)
	return appStateFromData(data)
}

func appStateFromData(data StoreData) AppState {
	return AppState{
		Columns:       BoardColumns,
		Roles:         rolePolicies(),
		Users:         cloneUsers(data.Users),
		Tasks:         cloneTasks(data.Tasks),
		Sprints:       cloneSprints(data.Sprints),
		Notifications: cloneNotifications(data.Notifications),
		Analytics:     buildAnalytics(data),
	}
}

func (s *Store) Authenticate(login, password string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return User{}, err
	}
	login = strings.TrimSpace(login)
	for _, user := range s.data.Users {
		if (strings.EqualFold(user.Login, login) || strings.EqualFold(user.Email, login) || user.ID == login) && user.Password == password {
			user.Password = ""
			return user, nil
		}
	}
	return User{}, errors.New("неверный пользователь или пароль")
}

func (s *Store) CreateUser(input UserInput) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return User{}, err
	}
	user, err := normalizeUserInput(input, User{})
	if err != nil {
		return User{}, err
	}
	if err := s.ensureUniqueUserLoginLocked(user.Login, ""); err != nil {
		return User{}, err
	}
	user.ID = s.nextID("user", "USR")
	if user.Password == "" {
		user.Password = defaultPassword(user.RoleID)
	}
	s.data.Users = append(s.data.Users, user)
	s.addNotification("user.created", fmt.Sprintf("Добавлен пользователь: %s", user.Name), "", user.ID)
	if err := s.saveLocked(); err != nil {
		return User{}, err
	}
	user.Password = ""
	return user, nil
}

func (s *Store) UpdateUser(id string, input UserInput) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return User{}, err
	}
	for i := range s.data.Users {
		if s.data.Users[i].ID != id {
			continue
		}
		user, err := normalizeUserInput(input, s.data.Users[i])
		if err != nil {
			return User{}, err
		}
		if err := s.ensureUniqueUserLoginLocked(user.Login, id); err != nil {
			return User{}, err
		}
		user.ID = id
		s.data.Users[i] = user
		s.addNotification("user.updated", fmt.Sprintf("Обновлен пользователь: %s", user.Name), "", user.ID)
		if err := s.saveLocked(); err != nil {
			return User{}, err
		}
		user.Password = ""
		return user, nil
	}
	return User{}, ErrNotFound("user", id)
}

func (s *Store) CreateTask(input TaskInput) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return Task{}, err
	}
	if err := normalizeTaskInput(&input); err != nil {
		return Task{}, err
	}

	now := time.Now().UTC()
	task := Task{
		ID:          s.nextID("task", "TASK"),
		Title:       input.Title,
		Description: input.Description,
		Status:      input.Status,
		Priority:    input.Priority,
		AssigneeID:  input.AssigneeID,
		ReporterID:  input.ReporterID,
		DueDate:     input.DueDate,
		StoryPoints: input.StoryPoints,
		SprintID:    input.SprintID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Comments:    []Comment{},
	}
	s.data.Tasks = append(s.data.Tasks, task)
	s.addNotification("task.created", fmt.Sprintf("Создана задача: %s", task.Title), task.ID, task.AssigneeID)

	return task, s.saveLocked()
}

func (s *Store) UpdateTask(id string, patch TaskPatch) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return Task{}, err
	}
	for i := range s.data.Tasks {
		if s.data.Tasks[i].ID != id {
			continue
		}

		beforeStatus := s.data.Tasks[i].Status
		applyTaskPatch(&s.data.Tasks[i], patch)
		if err := normalizeTask(&s.data.Tasks[i]); err != nil {
			return Task{}, err
		}
		if beforeStatus != s.data.Tasks[i].Status {
			s.data.Tasks[i].WorkDone = false
			s.data.Tasks[i].WorkDoneAt = time.Time{}
			s.addNotification(
				"task.status",
				fmt.Sprintf("Задача %s перемещена в %s", s.data.Tasks[i].Title, columnTitle(s.data.Tasks[i].Status)),
				s.data.Tasks[i].ID,
				s.data.Tasks[i].AssigneeID,
			)
		} else {
			s.addNotification("task.updated", fmt.Sprintf("Обновлена задача: %s", s.data.Tasks[i].Title), s.data.Tasks[i].ID, s.data.Tasks[i].AssigneeID)
		}
		s.data.Tasks[i].UpdatedAt = time.Now().UTC()
		if s.data.Tasks[i].Status == StatusDone && s.data.Tasks[i].CompletedAt.IsZero() {
			s.data.Tasks[i].CompletedAt = time.Now().UTC()
		}
		if s.data.Tasks[i].Status != StatusDone {
			s.data.Tasks[i].CompletedAt = time.Time{}
		}

		task := s.data.Tasks[i]
		return task, s.saveLocked()
	}
	return Task{}, ErrNotFound("task", id)
}

func (s *Store) AddComment(taskID string, input CommentInput) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return Task{}, err
	}
	text := strings.TrimSpace(input.Text)
	if text == "" {
		return Task{}, errors.New("comment text is required")
	}
	if input.AuthorID == "" && len(s.data.Users) > 0 {
		input.AuthorID = s.data.Users[0].ID
	}

	for i := range s.data.Tasks {
		if s.data.Tasks[i].ID != taskID {
			continue
		}
		comment := Comment{
			ID:        s.nextID("comment", "CMT"),
			TaskID:    taskID,
			AuthorID:  input.AuthorID,
			Text:      text,
			CreatedAt: time.Now().UTC(),
		}
		s.data.Tasks[i].Comments = append(s.data.Tasks[i].Comments, comment)
		s.data.Tasks[i].UpdatedAt = time.Now().UTC()
		s.addNotification("comment.created", fmt.Sprintf("Новый комментарий к задаче: %s", s.data.Tasks[i].Title), taskID, s.data.Tasks[i].AssigneeID)
		task := s.data.Tasks[i]
		return task, s.saveLocked()
	}
	return Task{}, ErrNotFound("task", taskID)
}

func (s *Store) CompleteTaskWork(taskID, authorID, text string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return Task{}, err
	}
	for i := range s.data.Tasks {
		if s.data.Tasks[i].ID != taskID {
			continue
		}

		now := time.Now().UTC()
		s.data.Tasks[i].WorkDone = true
		s.data.Tasks[i].WorkDoneAt = now
		s.data.Tasks[i].UpdatedAt = now

		text = strings.TrimSpace(text)
		if text != "" {
			comment := Comment{
				ID:        s.nextID("comment", "CMT"),
				TaskID:    taskID,
				AuthorID:  authorID,
				Text:      text,
				CreatedAt: now,
			}
			s.data.Tasks[i].Comments = append(s.data.Tasks[i].Comments, comment)
		}

		s.addNotification("task.work_done", fmt.Sprintf("Работа по задаче отмечена выполненной: %s", s.data.Tasks[i].Title), taskID, s.data.Tasks[i].AssigneeID)
		task := s.data.Tasks[i]
		return task, s.saveLocked()
	}
	return Task{}, ErrNotFound("task", taskID)
}

func (s *Store) DismissNotification(id string) (Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return Notification{}, err
	}
	for i := range s.data.Notifications {
		if s.data.Notifications[i].ID != id {
			continue
		}
		s.data.Notifications[i].Read = true
		note := s.data.Notifications[i]
		return note, s.saveLocked()
	}
	return Notification{}, ErrNotFound("notification", id)
}

func (s *Store) CreateSprint(input SprintInput) (Sprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return Sprint{}, err
	}
	if err := normalizeSprintInput(&input); err != nil {
		return Sprint{}, err
	}
	sprint := Sprint{
		ID:            s.nextID("sprint", "SPR"),
		Name:          input.Name,
		Goal:          input.Goal,
		StartDate:     input.StartDate,
		EndDate:       input.EndDate,
		Status:        input.Status,
		Retrospective: input.Retrospective,
	}
	s.data.Sprints = append(s.data.Sprints, sprint)
	s.addNotification("sprint.created", fmt.Sprintf("Создан спринт: %s", sprint.Name), "", "")
	return sprint, s.saveLocked()
}

func (s *Store) UpdateSprint(id string, input SprintInput) (Sprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshLocked(); err != nil {
		return Sprint{}, err
	}
	if err := normalizeSprintInput(&input); err != nil {
		return Sprint{}, err
	}
	for i := range s.data.Sprints {
		if s.data.Sprints[i].ID != id {
			continue
		}
		s.data.Sprints[i].Name = input.Name
		s.data.Sprints[i].Goal = input.Goal
		s.data.Sprints[i].StartDate = input.StartDate
		s.data.Sprints[i].EndDate = input.EndDate
		s.data.Sprints[i].Status = input.Status
		s.data.Sprints[i].Retrospective = input.Retrospective
		s.addNotification("sprint.updated", fmt.Sprintf("Обновлен спринт: %s", input.Name), "", "")
		sprint := s.data.Sprints[i]
		return sprint, s.saveLocked()
	}
	return Sprint{}, ErrNotFound("sprint", id)
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		s.data = seedData()
		s.ensureUserRolesLocked()
		s.ensureUserLoginsLocked()
		s.ensureNextCountersLocked()
		return nil
	}
	if err := s.loadPostgresLocked(); err != nil {
		return err
	}
	if s.data.Next == nil {
		s.data.Next = map[string]int{}
	}
	s.ensureUserRolesLocked()
	s.ensureUserLoginsLocked()
	s.ensureNextCountersLocked()
	return nil
}

func (s *Store) refreshLocked() error {
	if s.db == nil {
		return nil
	}
	if err := s.loadPostgresLocked(); err != nil {
		return err
	}
	if s.data.Next == nil {
		s.data.Next = map[string]int{}
	}
	s.ensureUserRolesLocked()
	s.ensureUserLoginsLocked()
	s.ensureNextCountersLocked()
	return nil
}

func (s *Store) saveLocked() error {
	if s.db == nil {
		return nil
	}
	return s.savePostgresLocked()
}

func (s *Store) nextID(key, prefix string) string {
	if s.data.Next == nil {
		s.data.Next = map[string]int{}
	}
	s.data.Next[key]++
	return fmt.Sprintf("%s-%03d", prefix, s.data.Next[key])
}

func (s *Store) addNotification(kind, message, taskID, userID string) {
	now := time.Now().UTC()
	notes := make([]Notification, 0, len(s.data.Users))
	for _, user := range s.data.Users {
		notes = append(notes, Notification{
			ID:        s.nextID("notification", "NTF"),
			Type:      kind,
			Message:   message,
			TaskID:    taskID,
			UserID:    user.ID,
			CreatedAt: now,
		})
	}
	if len(notes) == 0 {
		notes = append(notes, Notification{
			ID:        s.nextID("notification", "NTF"),
			Type:      kind,
			Message:   message,
			TaskID:    taskID,
			UserID:    userID,
			CreatedAt: now,
		})
	}
	s.data.Notifications = append(notes, s.data.Notifications...)
	s.trimNotificationsLocked(50)
}

func (s *Store) trimNotificationsLocked(limitPerUser int) {
	seen := map[string]int{}
	trimmed := make([]Notification, 0, len(s.data.Notifications))
	for _, note := range s.data.Notifications {
		key := note.UserID
		seen[key]++
		if seen[key] <= limitPerUser {
			trimmed = append(trimmed, note)
		}
	}
	s.data.Notifications = trimmed
}

func normalizeTaskInput(input *TaskInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return errors.New("task title is required")
	}
	if input.Status == "" {
		input.Status = StatusBacklog
	}
	if input.Priority == "" {
		input.Priority = "medium"
	}
	if input.StoryPoints < 0 {
		input.StoryPoints = 0
	}
	return validateStatus(input.Status)
}

func normalizeTask(task *Task) error {
	task.Title = strings.TrimSpace(task.Title)
	if task.Title == "" {
		return errors.New("task title is required")
	}
	if task.Priority == "" {
		task.Priority = "medium"
	}
	if task.StoryPoints < 0 {
		task.StoryPoints = 0
	}
	return validateStatus(task.Status)
}

func normalizeSprintInput(input *SprintInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return errors.New("sprint name is required")
	}
	if input.Status == "" {
		input.Status = "planned"
	}
	switch input.Status {
	case "planned", "active", "closed":
		return nil
	default:
		return fmt.Errorf("unknown sprint status: %s", input.Status)
	}
}

func normalizeUserInput(input UserInput, current User) (User, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return User{}, errors.New("user name is required")
	}
	roleID := normalizeRoleID(input.RoleID, "")
	if roleID != RoleAdmin && roleID != RoleManager && roleID != RoleDeveloper {
		return User{}, fmt.Errorf("unknown user role: %s", input.RoleID)
	}
	login := normalizeLogin(input.Login)
	if login == "" {
		login = normalizeLogin(current.Login)
	}
	if login == "" {
		login = loginFromEmail(input.Email)
	}
	if login == "" {
		return User{}, errors.New("user login is required")
	}
	password := strings.TrimSpace(input.Password)
	if password == "" {
		password = current.Password
	}
	return User{
		Login:    login,
		Name:     name,
		RoleID:   roleID,
		Role:     roleName(roleID),
		Email:    strings.TrimSpace(input.Email),
		Password: password,
	}, nil
}

func validateStatus(status string) error {
	for _, col := range BoardColumns {
		if col.ID == status {
			return nil
		}
	}
	return fmt.Errorf("unknown task status: %s", status)
}

func normalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

func loginFromEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	name, _, ok := strings.Cut(email, "@")
	if !ok {
		return normalizeLogin(email)
	}
	return normalizeLogin(name)
}

func (s *Store) ensureUniqueUserLoginLocked(login, currentID string) error {
	for _, user := range s.data.Users {
		if user.ID != currentID && strings.EqualFold(user.Login, login) {
			return fmt.Errorf("login already exists: %s", login)
		}
	}
	return nil
}

func (s *Store) ensureNextCountersLocked() {
	if s.data.Next == nil {
		s.data.Next = map[string]int{}
	}
	s.data.Next["user"] = maxIDCounter(s.data.Users, s.data.Next["user"], func(user User) string { return user.ID }, "USR")
	s.data.Next["task"] = maxIDCounter(s.data.Tasks, s.data.Next["task"], func(task Task) string { return task.ID }, "TASK")
	s.data.Next["sprint"] = maxIDCounter(s.data.Sprints, s.data.Next["sprint"], func(sprint Sprint) string { return sprint.ID }, "SPR")
	s.data.Next["notification"] = maxIDCounter(s.data.Notifications, s.data.Next["notification"], func(note Notification) string { return note.ID }, "NTF")
	for _, task := range s.data.Tasks {
		s.data.Next["comment"] = maxIDCounter(task.Comments, s.data.Next["comment"], func(comment Comment) string { return comment.ID }, "CMT")
	}
}

func maxIDCounter[T any](items []T, current int, id func(T) string, prefix string) int {
	maxValue := current
	for _, item := range items {
		raw := strings.TrimPrefix(id(item), prefix+"-")
		value, err := strconv.Atoi(raw)
		if err == nil && value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func applyTaskPatch(task *Task, patch TaskPatch) {
	if patch.Title != nil {
		task.Title = *patch.Title
	}
	if patch.Description != nil {
		task.Description = *patch.Description
	}
	if patch.Status != nil {
		task.Status = *patch.Status
	}
	if patch.Priority != nil {
		task.Priority = *patch.Priority
	}
	if patch.AssigneeID != nil {
		task.AssigneeID = *patch.AssigneeID
	}
	if patch.ReporterID != nil {
		task.ReporterID = *patch.ReporterID
	}
	if patch.DueDate != nil {
		task.DueDate = *patch.DueDate
	}
	if patch.StoryPoints != nil {
		task.StoryPoints = *patch.StoryPoints
	}
	if patch.SprintID != nil {
		task.SprintID = *patch.SprintID
	}
}

func buildAnalytics(data StoreData) Analytics {
	statusCounts := map[string]int{}
	priorityCounts := map[string]int{}
	loadByUser := map[string]*TeamMemberLoad{}
	userNames := map[string]string{}
	completed := 0
	active := 0
	velocity := 0
	cycleHours := 0.0
	cycledTasks := 0
	wip := 0

	for _, user := range data.Users {
		userNames[user.ID] = user.Name
		loadByUser[user.ID] = &TeamMemberLoad{UserID: user.ID, Name: user.Name, Role: user.Role}
	}

	for _, task := range data.Tasks {
		statusCounts[task.Status]++
		priorityCounts[task.Priority]++
		if task.Status == StatusDone {
			completed++
			velocity += task.StoryPoints
			if !task.CompletedAt.IsZero() {
				cycleHours += task.CompletedAt.Sub(task.CreatedAt).Hours()
				cycledTasks++
			}
		} else {
			active++
		}
		if task.Status == StatusInProgress || task.Status == StatusReview {
			wip++
		}
		if member := loadByUser[task.AssigneeID]; member != nil {
			if task.Status == StatusDone {
				member.DoneTasks++
			} else {
				member.ActiveTasks++
				member.StoryPoints += task.StoryPoints
			}
		}
	}

	load := make([]TeamMemberLoad, 0, len(loadByUser))
	for _, item := range loadByUser {
		load = append(load, *item)
	}
	sort.Slice(load, func(i, j int) bool {
		if load[i].ActiveTasks == load[j].ActiveTasks {
			return load[i].Name < load[j].Name
		}
		return load[i].ActiveTasks > load[j].ActiveTasks
	})

	progress := make([]SprintProgress, 0, len(data.Sprints))
	for _, sprint := range data.Sprints {
		item := SprintProgress{SprintID: sprint.ID, Name: sprint.Name, Status: sprint.Status}
		for _, task := range data.Tasks {
			if task.SprintID != sprint.ID {
				continue
			}
			item.TotalTasks++
			if task.Status == StatusDone {
				item.DoneTasks++
			}
		}
		if item.TotalTasks > 0 {
			item.CompletionRatio = float64(item.DoneTasks) / float64(item.TotalTasks)
		}
		progress = append(progress, item)
	}

	averageCycle := 0.0
	if cycledTasks > 0 {
		averageCycle = cycleHours / float64(cycledTasks)
	}

	return Analytics{
		StatusCounts:      statusCounts,
		PriorityCounts:    priorityCounts,
		CompletedTasks:    completed,
		ActiveTasks:       active,
		VelocityPoints:    velocity,
		AverageCycleHours: averageCycle,
		TeamLoad:          load,
		SprintProgress:    progress,
		RecentActivity:    recent(data.Notifications, 8),
		WorkInProgress:    wip,
		DueSoonTasks:      dueSoon(data.Tasks, userNames),
	}
}

func recent(notes []Notification, limit int) []Notification {
	items := make([]Notification, 0, limit)
	for _, note := range notes {
		if note.Read {
			continue
		}
		items = append(items, note)
		if len(items) == limit {
			return items
		}
	}
	return items
}

func notificationsForUser(notes []Notification, userID string) []Notification {
	if userID == "" {
		return notes
	}
	items := make([]Notification, 0, len(notes))
	for _, note := range notes {
		if note.UserID == "" || note.UserID == userID {
			items = append(items, note)
		}
	}
	return items
}

func dueSoon(tasks []Task, userNames map[string]string) []TaskDueIndicator {
	now := time.Now()
	limit := now.AddDate(0, 0, 7)
	items := []TaskDueIndicator{}
	for _, task := range tasks {
		if task.DueDate == "" || task.Status == StatusDone {
			continue
		}
		due, err := time.Parse("2006-01-02", task.DueDate)
		if err != nil {
			continue
		}
		if due.Before(now) || due.Equal(limit) || due.Before(limit) {
			items = append(items, TaskDueIndicator{
				TaskID:   task.ID,
				Title:    task.Title,
				DueDate:  task.DueDate,
				Assignee: userNames[task.AssigneeID],
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DueDate < items[j].DueDate })
	if len(items) > 6 {
		return items[:6]
	}
	return items
}

func columnTitle(status string) string {
	for _, col := range BoardColumns {
		if col.ID == status {
			return col.Title
		}
	}
	return status
}

func ErrNotFound(kind, id string) error {
	return fmt.Errorf("%s not found: %s", kind, id)
}

func cloneUsers(items []User) []User {
	out := make([]User, len(items))
	copy(out, items)
	for i := range out {
		out[i].Password = ""
	}
	return out
}

func cloneSprints(items []Sprint) []Sprint {
	out := make([]Sprint, len(items))
	copy(out, items)
	return out
}

func cloneNotifications(items []Notification) []Notification {
	out := make([]Notification, len(items))
	copy(out, items)
	return out
}

func cloneTasks(items []Task) []Task {
	out := make([]Task, len(items))
	for i, task := range items {
		out[i] = task
		out[i].Comments = append([]Comment(nil), task.Comments...)
	}
	return out
}

func logStoreError(context string, err error) {
	if err != nil {
		log.Printf("%s: %v", context, err)
	}
}
