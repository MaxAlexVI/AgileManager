package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"agile-manager/internal/analytics"
	"agile-manager/internal/notifications"
	"agile-manager/internal/shared"
	"agile-manager/internal/sprints"
	"agile-manager/internal/tasks"
	"agile-manager/internal/users"
)

type ServiceURLs struct {
	Users         string
	Tasks         string
	Sprints       string
	Notifications string
	Analytics     string
}

type Server struct {
	store         *shared.Store
	users         *users.Service
	tasks         *tasks.Service
	sprints       *sprints.Service
	notifications *notifications.Service
	analytics     *analytics.Service
	static        http.Handler
	mu            sync.RWMutex
	sessions      map[string]string
	eventMu       sync.RWMutex
	clients       map[chan string]struct{}
	remotes       ServiceURLs
}

func NewServer(store *shared.Store, staticDir string) *Server {
	return NewServerWithRemotes(store, staticDir, ServiceURLs{})
}

func NewServerWithRemotes(store *shared.Store, staticDir string, remotes ServiceURLs) *Server {
	return &Server{
		store:         store,
		users:         users.New(store),
		tasks:         tasks.New(store),
		sprints:       sprints.New(store),
		notifications: notifications.New(store),
		analytics:     analytics.New(store),
		static:        http.FileServer(http.Dir(staticDir)),
		sessions:      map[string]string{},
		clients:       map[chan string]struct{}{},
		remotes:       normalizeServiceURLs(remotes),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/state", s.state)
	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("POST /api/users", s.createUser)
	mux.HandleFunc("PUT /api/users/{id}", s.updateUser)
	mux.HandleFunc("POST /api/tasks", s.createTask)
	mux.HandleFunc("PATCH /api/tasks/{id}", s.updateTask)
	mux.HandleFunc("POST /api/tasks/{id}/complete", s.completeTaskWork)
	mux.HandleFunc("POST /api/tasks/{id}/comments", s.addComment)
	mux.HandleFunc("PATCH /api/notifications/{id}/read", s.dismissNotification)
	mux.HandleFunc("POST /api/sprints", s.createSprint)
	mux.HandleFunc("PUT /api/sprints/{id}", s.updateSprint)
	mux.HandleFunc("GET /api/reports/team-load", s.teamLoad)
	mux.HandleFunc("/", s.web)
	return logging(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input shared.LoginInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	login := input.Login
	if login == "" {
		login = input.UserID
	}
	user, err := s.store.Authenticate(login, input.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	token, err := newSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.mu.Lock()
	s.sessions[token] = user.ID
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, shared.LoginResponse{Token: token, User: user})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token != "" {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	writeJSON(w, http.StatusOK, s.store.StateForUser(actor.ID))
}

func (s *Server) teamLoad(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Analytics) {
		return
	}
	load, err := s.analytics.TeamLoad(actor)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusOK, load)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Users) {
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
	s.broadcastStateChanged()
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Users) {
		return
	}
	var input shared.UserInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.users.Update(actor, r.PathValue("id"), input)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "права") || strings.Contains(err.Error(), "РїСЂР°РІР°") {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	s.broadcastStateChanged()
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Tasks) {
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
	s.broadcastStateChanged()
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Tasks) {
		return
	}
	var patch shared.TaskPatch
	if err := readJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, err := s.tasks.Update(actor, r.PathValue("id"), patch)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "права") {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	s.broadcastStateChanged()
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) completeTaskWork(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Tasks) {
		return
	}
	var input shared.CompleteTaskInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, err := s.tasks.CompleteWork(actor, r.PathValue("id"), input)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "права") {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	s.broadcastStateChanged()
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Tasks) {
		return
	}
	var input shared.CommentInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, err := s.tasks.Comment(actor, r.PathValue("id"), input)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "права") {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	s.broadcastStateChanged()
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) dismissNotification(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Notifications) {
		return
	}
	note, err := s.notifications.Dismiss(actor, r.PathValue("id"))
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "права") {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	s.broadcastStateChanged()
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) createSprint(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Sprints) {
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
	s.broadcastStateChanged()
	writeJSON(w, http.StatusCreated, sprint)
}

func (s *Server) updateSprint(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if s.proxyToService(w, r, actor, s.remotes.Sprints) {
		return
	}
	var input shared.SprintInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sprint, err := s.sprints.Update(actor, r.PathValue("id"), input)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "права") {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	s.broadcastStateChanged()
	writeJSON(w, http.StatusOK, sprint)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if _, err := s.actorFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming is not supported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	client := make(chan string, 4)
	s.eventMu.Lock()
	s.clients[client] = struct{}{}
	s.eventMu.Unlock()
	defer func() {
		s.eventMu.Lock()
		delete(s.clients, client)
		close(client)
		s.eventMu.Unlock()
	}()

	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case event := <-client:
			fmt.Fprintf(w, "event: state\ndata: %s\n\n", event)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) web(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, errors.New("endpoint not found"))
		return
	}
	s.static.ServeHTTP(w, r)
}

func (s *Server) actor(r *http.Request) (shared.User, error) {
	token := bearerToken(r)
	return s.actorFromToken(token)
}

func (s *Server) actorFromRequest(r *http.Request) (shared.User, error) {
	token := bearerToken(r)
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	return s.actorFromToken(token)
}

func (s *Server) actorFromToken(token string) (shared.User, error) {
	if token == "" {
		return shared.User{}, errors.New("требуется авторизация")
	}
	s.mu.RLock()
	actorID := s.sessions[token]
	s.mu.RUnlock()
	if actorID == "" {
		return shared.User{}, errors.New("сессия истекла или недействительна")
	}
	state := s.store.State()
	if user, ok := shared.FindUser(state.Users, actorID); ok {
		return user, nil
	}
	return shared.User{}, errors.New("unknown user")
}

func (s *Server) broadcastStateChanged() {
	payload := fmt.Sprintf(`{"at":%q}`, time.Now().UTC().Format(time.RFC3339Nano))
	s.eventMu.RLock()
	defer s.eventMu.RUnlock()
	for client := range s.clients {
		select {
		case client <- payload:
		default:
		}
	}
}

func (s *Server) proxyToService(w http.ResponseWriter, r *http.Request, actor shared.User, baseURL string) bool {
	if baseURL == "" {
		return false
	}
	target, err := url.JoinPath(baseURL, r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return true
	}
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return true
	}
	request.Header = r.Header.Clone()
	request.Header.Set("X-Actor-ID", actor.ID)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return true
	}
	defer response.Body.Close()

	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	if _, err := io.Copy(w, response.Body); err != nil {
		log.Printf("proxy response: %v", err)
	}
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusBadRequest && r.Method != http.MethodGet {
		s.broadcastStateChanged()
	}
	return true
}

func normalizeServiceURLs(urls ServiceURLs) ServiceURLs {
	return ServiceURLs{
		Users:         strings.TrimRight(strings.TrimSpace(urls.Users), "/"),
		Tasks:         strings.TrimRight(strings.TrimSpace(urls.Tasks), "/"),
		Sprints:       strings.TrimRight(strings.TrimSpace(urls.Sprints), "/"),
		Notifications: strings.TrimRight(strings.TrimSpace(urls.Notifications), "/"),
		Analytics:     strings.TrimRight(strings.TrimSpace(urls.Analytics), "/"),
	}
}

func bearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
}

func newSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
