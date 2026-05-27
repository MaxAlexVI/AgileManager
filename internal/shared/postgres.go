package shared

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

func (s *Store) loadPostgresLocked() error {
	ctx := context.Background()
	if err := s.db.PingContext(ctx); err != nil {
		return err
	}
	if err := s.ensurePostgresSchema(ctx); err != nil {
		return err
	}

	var usersCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&usersCount); err != nil {
		return err
	}
	if usersCount == 0 {
		s.data = seedData()
		s.ensureUserRolesLocked()
		s.ensureUserLoginsLocked()
		s.ensureNextCountersLocked()
		return s.savePostgresLocked()
	}

	data := StoreData{Next: map[string]int{}}
	users, err := queryUsers(ctx, s.db)
	if err != nil {
		return err
	}
	data.Users = users

	sprints, err := querySprints(ctx, s.db)
	if err != nil {
		return err
	}
	data.Sprints = sprints

	tasks, err := queryTasks(ctx, s.db)
	if err != nil {
		return err
	}
	data.Tasks = tasks

	comments, err := queryComments(ctx, s.db)
	if err != nil {
		return err
	}
	commentsByTask := map[string][]Comment{}
	for _, comment := range comments {
		commentsByTask[comment.TaskID] = append(commentsByTask[comment.TaskID], comment)
	}
	for i := range data.Tasks {
		data.Tasks[i].Comments = commentsByTask[data.Tasks[i].ID]
		if data.Tasks[i].Comments == nil {
			data.Tasks[i].Comments = []Comment{}
		}
	}

	notifications, err := queryNotifications(ctx, s.db)
	if err != nil {
		return err
	}
	data.Notifications = notifications

	next, err := queryCounters(ctx, s.db)
	if err != nil {
		return err
	}
	data.Next = next

	s.data = data
	return nil
}

func (s *Store) ensurePostgresSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			login TEXT UNIQUE,
			name TEXT NOT NULL,
			role_id TEXT NOT NULL,
			role TEXT NOT NULL,
			email TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL
		)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS login TEXT`,
		`CREATE UNIQUE INDEX IF NOT EXISTS users_login_unique ON users (login) WHERE login <> ''`,
		`CREATE TABLE IF NOT EXISTS sprints (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			goal TEXT NOT NULL DEFAULT '',
			start_date TEXT NOT NULL DEFAULT '',
			end_date TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			retrospective TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			priority TEXT NOT NULL,
			assignee_id TEXT NOT NULL DEFAULT '',
			reporter_id TEXT NOT NULL DEFAULT '',
			due_date TEXT NOT NULL DEFAULT '',
			story_points INTEGER NOT NULL DEFAULT 0,
			sprint_id TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ,
			work_done BOOLEAN NOT NULL DEFAULT false,
			work_done_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			author_id TEXT NOT NULL DEFAULT '',
			text TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS comments_task_id_idx ON comments (task_id)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			message TEXT NOT NULL,
			task_id TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT '',
			is_read BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS counters (
			key TEXT PRIMARY KEY,
			value INTEGER NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) savePostgresLocked() error {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, statement := range []string{
		`DELETE FROM comments`,
		`DELETE FROM notifications`,
		`DELETE FROM tasks`,
		`DELETE FROM sprints`,
		`DELETE FROM users`,
		`DELETE FROM counters`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	for _, user := range s.data.Users {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, login, name, role_id, role, email, password) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			user.ID, user.Login, user.Name, user.RoleID, user.Role, user.Email, user.Password); err != nil {
			return err
		}
	}
	for _, sprint := range s.data.Sprints {
		if _, err := tx.ExecContext(ctx, `INSERT INTO sprints (id, name, goal, start_date, end_date, status, retrospective) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			sprint.ID, sprint.Name, sprint.Goal, sprint.StartDate, sprint.EndDate, sprint.Status, sprint.Retrospective); err != nil {
			return err
		}
	}
	for _, task := range s.data.Tasks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tasks (id, title, description, status, priority, assignee_id, reporter_id, due_date, story_points, sprint_id, created_at, updated_at, completed_at, work_done, work_done_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
			task.ID, task.Title, task.Description, task.Status, task.Priority, task.AssigneeID, task.ReporterID, task.DueDate, task.StoryPoints, task.SprintID, task.CreatedAt, task.UpdatedAt, nullableTime(task.CompletedAt), task.WorkDone, nullableTime(task.WorkDoneAt)); err != nil {
			return err
		}
		for _, comment := range task.Comments {
			if _, err := tx.ExecContext(ctx, `INSERT INTO comments (id, task_id, author_id, text, created_at) VALUES ($1, $2, $3, $4, $5)`,
				comment.ID, comment.TaskID, comment.AuthorID, comment.Text, comment.CreatedAt); err != nil {
				return err
			}
		}
	}
	for _, note := range s.data.Notifications {
		if _, err := tx.ExecContext(ctx, `INSERT INTO notifications (id, kind, message, task_id, user_id, is_read, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			note.ID, note.Type, note.Message, note.TaskID, note.UserID, note.Read, note.CreatedAt); err != nil {
			return err
		}
	}
	for key, value := range s.data.Next {
		if _, err := tx.ExecContext(ctx, `INSERT INTO counters (key, value) VALUES ($1, $2)`, key, value); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func queryUsers(ctx context.Context, db *sql.DB) ([]User, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, COALESCE(login, ''), name, role_id, role, email, password FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Login, &user.Name, &user.RoleID, &user.Role, &user.Email, &user.Password); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func querySprints(ctx context.Context, db *sql.DB) ([]Sprint, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, goal, start_date, end_date, status, retrospective FROM sprints ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sprints []Sprint
	for rows.Next() {
		var sprint Sprint
		if err := rows.Scan(&sprint.ID, &sprint.Name, &sprint.Goal, &sprint.StartDate, &sprint.EndDate, &sprint.Status, &sprint.Retrospective); err != nil {
			return nil, err
		}
		sprints = append(sprints, sprint)
	}
	return sprints, rows.Err()
}

func queryTasks(ctx context.Context, db *sql.DB) ([]Task, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, title, description, status, priority, assignee_id, reporter_id, due_date, story_points, sprint_id, created_at, updated_at, completed_at, work_done, work_done_at FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		var completedAt, workDoneAt sql.NullTime
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority, &task.AssigneeID, &task.ReporterID, &task.DueDate, &task.StoryPoints, &task.SprintID, &task.CreatedAt, &task.UpdatedAt, &completedAt, &task.WorkDone, &workDoneAt); err != nil {
			return nil, err
		}
		task.CompletedAt = timeFromNull(completedAt)
		task.WorkDoneAt = timeFromNull(workDoneAt)
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func queryComments(ctx context.Context, db *sql.DB) ([]Comment, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, task_id, author_id, text, created_at FROM comments ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.ID, &comment.TaskID, &comment.AuthorID, &comment.Text, &comment.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func queryNotifications(ctx context.Context, db *sql.DB) ([]Notification, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, kind, message, task_id, user_id, is_read, created_at FROM notifications ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var note Notification
		if err := rows.Scan(&note.ID, &note.Type, &note.Message, &note.TaskID, &note.UserID, &note.Read, &note.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, note)
	}
	return notifications, rows.Err()
}

func queryCounters(ctx context.Context, db *sql.DB) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `SELECT key, value FROM counters`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counters := map[string]int{}
	for rows.Next() {
		var key string
		var value int
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		counters[key] = value
	}
	return counters, rows.Err()
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func timeFromNull(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func postgresError(message string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return fmt.Errorf("%s: %w", message, err)
}
