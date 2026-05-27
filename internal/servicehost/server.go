package servicehost

import (
	"errors"
	"net/http"
	"strings"

	"agile-manager/internal/analytics"
	"agile-manager/internal/notifications"
	"agile-manager/internal/shared"
	"agile-manager/internal/sprints"
	"agile-manager/internal/tasks"
	"agile-manager/internal/users"
)

type DomainService string

const (
	DomainUsers         DomainService = "users"
	DomainTasks         DomainService = "tasks"
	DomainSprints       DomainService = "sprints"
	DomainNotifications DomainService = "notifications"
	DomainAnalytics     DomainService = "analytics"
)

type DomainServer struct {
	store         *shared.Store
	users         *users.Service
	tasks         *tasks.Service
	sprints       *sprints.Service
	notifications *notifications.Service
	analytics     *analytics.Service
	domain        DomainService
}

func NewDomainServer(store *shared.Store, domain DomainService) *DomainServer {
	return &DomainServer{
		store:         store,
		users:         users.New(store),
		tasks:         tasks.New(store),
		sprints:       sprints.New(store),
		notifications: notifications.New(store),
		analytics:     analytics.New(store),
		domain:        domain,
	}
}

func (s *DomainServer) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	switch s.domain {
	case DomainUsers:
		mux.HandleFunc("POST /api/users", s.createUser)
		mux.HandleFunc("PUT /api/users/{id}", s.updateUser)
	case DomainTasks:
		mux.HandleFunc("POST /api/tasks", s.createTask)
		mux.HandleFunc("PATCH /api/tasks/{id}", s.updateTask)
		mux.HandleFunc("POST /api/tasks/{id}/complete", s.completeTaskWork)
		mux.HandleFunc("POST /api/tasks/{id}/comments", s.addComment)
	case DomainSprints:
		mux.HandleFunc("POST /api/sprints", s.createSprint)
		mux.HandleFunc("PUT /api/sprints/{id}", s.updateSprint)
	case DomainNotifications:
		mux.HandleFunc("PATCH /api/notifications/{id}/read", s.dismissNotification)
	case DomainAnalytics:
		mux.HandleFunc("GET /api/reports/team-load", s.teamLoad)
	}
	mux.HandleFunc("/", s.notFound)
	return logging(mux)
}

func (s *DomainServer) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": string(s.domain)})
}

func (s *DomainServer) createUser(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var input shared.UserInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.users.Create(actor, input)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (s *DomainServer) updateUser(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var input shared.UserInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.users.Update(actor, r.PathValue("id"), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *DomainServer) createTask(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var input shared.TaskInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, err := s.tasks.Create(actor, input)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *DomainServer) updateTask(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var patch shared.TaskPatch
	if err := readJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, err := s.tasks.Update(actor, r.PathValue("id"), patch)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *DomainServer) completeTaskWork(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var input shared.CompleteTaskInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, err := s.tasks.CompleteWork(actor, r.PathValue("id"), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *DomainServer) addComment(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var input shared.CommentInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, err := s.tasks.Comment(actor, r.PathValue("id"), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *DomainServer) createSprint(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var input shared.SprintInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sprint, err := s.sprints.Create(actor, input)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusCreated, sprint)
}

func (s *DomainServer) updateSprint(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var input shared.SprintInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sprint, err := s.sprints.Update(actor, r.PathValue("id"), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sprint)
}

func (s *DomainServer) dismissNotification(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	note, err := s.notifications.Dismiss(actor, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *DomainServer) teamLoad(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	load, err := s.analytics.TeamLoad(actor)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusOK, load)
}

func (s *DomainServer) actor(r *http.Request) (shared.User, error) {
	actorID := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if actorID == "" {
		return shared.User{}, errors.New("actor header is required")
	}
	state := s.store.State()
	if user, ok := shared.FindUser(state.Users, actorID); ok {
		return user, nil
	}
	return shared.User{}, errors.New("unknown actor")
}

func (s *DomainServer) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, errors.New("endpoint not found in "+string(s.domain)+" service"))
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	} else if strings.Contains(err.Error(), "права") {
		status = http.StatusForbidden
	}
	writeError(w, status, err)
}
