package shared

import "time"

const (
	StatusBacklog    = "backlog"
	StatusTodo       = "todo"
	StatusInProgress = "in_progress"
	StatusReview     = "review"
	StatusDone       = "done"
)

var BoardColumns = []Column{
	{ID: StatusBacklog, Title: "Backlog"},
	{ID: StatusTodo, Title: "To Do"},
	{ID: StatusInProgress, Title: "In Progress"},
	{ID: StatusReview, Title: "Review"},
	{ID: StatusDone, Title: "Done"},
}

type Column struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type User struct {
	ID       string `json:"id"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	RoleID   string `json:"roleId"`
	Role     string `json:"role"`
	Email    string `json:"email"`
	Password string `json:"password,omitempty"`
}

type UserInput struct {
	Login    string `json:"login"`
	Name     string `json:"name"`
	RoleID   string `json:"roleId"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RolePolicy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	AssigneeID  string    `json:"assigneeId"`
	ReporterID  string    `json:"reporterId"`
	DueDate     string    `json:"dueDate"`
	StoryPoints int       `json:"storyPoints"`
	SprintID    string    `json:"sprintId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
	WorkDone    bool      `json:"workDone"`
	WorkDoneAt  time.Time `json:"workDoneAt,omitempty"`
	Comments    []Comment `json:"comments"`
}

type Comment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	AuthorID  string    `json:"authorId"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type Sprint struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Goal          string `json:"goal"`
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	Status        string `json:"status"`
	Retrospective string `json:"retrospective"`
}

type Notification struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	TaskID    string    `json:"taskId,omitempty"`
	UserID    string    `json:"userId,omitempty"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

type StoreData struct {
	Users         []User         `json:"users"`
	Tasks         []Task         `json:"tasks"`
	Sprints       []Sprint       `json:"sprints"`
	Notifications []Notification `json:"notifications"`
	Next          map[string]int `json:"next"`
}

type AppState struct {
	Columns       []Column       `json:"columns"`
	Roles         []RolePolicy   `json:"roles"`
	Users         []User         `json:"users"`
	Tasks         []Task         `json:"tasks"`
	Sprints       []Sprint       `json:"sprints"`
	Notifications []Notification `json:"notifications"`
	Analytics     Analytics      `json:"analytics"`
}

type Analytics struct {
	StatusCounts      map[string]int     `json:"statusCounts"`
	PriorityCounts    map[string]int     `json:"priorityCounts"`
	CompletedTasks    int                `json:"completedTasks"`
	ActiveTasks       int                `json:"activeTasks"`
	VelocityPoints    int                `json:"velocityPoints"`
	AverageCycleHours float64            `json:"averageCycleHours"`
	TeamLoad          []TeamMemberLoad   `json:"teamLoad"`
	SprintProgress    []SprintProgress   `json:"sprintProgress"`
	RecentActivity    []Notification     `json:"recentActivity"`
	WorkInProgress    int                `json:"workInProgress"`
	DueSoonTasks      []TaskDueIndicator `json:"dueSoonTasks"`
}

type TeamMemberLoad struct {
	UserID      string `json:"userId"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	ActiveTasks int    `json:"activeTasks"`
	DoneTasks   int    `json:"doneTasks"`
	StoryPoints int    `json:"storyPoints"`
}

type SprintProgress struct {
	SprintID        string  `json:"sprintId"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	TotalTasks      int     `json:"totalTasks"`
	DoneTasks       int     `json:"doneTasks"`
	CompletionRatio float64 `json:"completionRatio"`
}

type TaskDueIndicator struct {
	TaskID   string `json:"taskId"`
	Title    string `json:"title"`
	DueDate  string `json:"dueDate"`
	Assignee string `json:"assignee"`
}

type TaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	AssigneeID  string `json:"assigneeId"`
	ReporterID  string `json:"reporterId"`
	DueDate     string `json:"dueDate"`
	StoryPoints int    `json:"storyPoints"`
	SprintID    string `json:"sprintId"`
}

type TaskPatch struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assigneeId"`
	ReporterID  *string `json:"reporterId"`
	DueDate     *string `json:"dueDate"`
	StoryPoints *int    `json:"storyPoints"`
	SprintID    *string `json:"sprintId"`
}

type CommentInput struct {
	AuthorID string `json:"authorId"`
	Text     string `json:"text"`
}

type SprintInput struct {
	Name          string `json:"name"`
	Goal          string `json:"goal"`
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	Status        string `json:"status"`
	Retrospective string `json:"retrospective"`
}

type LoginInput struct {
	Login    string `json:"login"`
	UserID   string `json:"userId"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type CompleteTaskInput struct {
	Comment string `json:"comment"`
}
