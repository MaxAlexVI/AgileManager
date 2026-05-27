package shared

import (
	"fmt"
	"strings"
)

const (
	RoleAdmin     = "admin"
	RoleManager   = "manager"
	RoleDeveloper = "developer"
)

const (
	PermManageUsers     = "manage_users"
	PermManageSprints   = "manage_sprints"
	PermCreateTasks     = "create_tasks"
	PermEditAnyTask     = "edit_any_task"
	PermEditOwnTask     = "edit_own_task"
	PermCommentTasks    = "comment_tasks"
	PermCompleteOwnTask = "complete_own_task"
	PermViewAnalytics   = "view_analytics"
	PermDismissActivity = "dismiss_activity"
)

var RoleCatalog = []RolePolicy{
	{
		ID:          RoleAdmin,
		Name:        "Администратор",
		Description: "Полный доступ к пользователям, задачам, спринтам и отчетам.",
		Permissions: []string{
			PermManageUsers,
			PermManageSprints,
			PermCreateTasks,
			PermEditAnyTask,
			PermCommentTasks,
			PermViewAnalytics,
			PermDismissActivity,
		},
	},
	{
		ID:          RoleManager,
		Name:        "Руководитель",
		Description: "Планирует спринты, создает задачи, распределяет работу и смотрит аналитику.",
		Permissions: []string{
			PermManageUsers,
			PermManageSprints,
			PermCreateTasks,
			PermEditAnyTask,
			PermCommentTasks,
			PermViewAnalytics,
			PermDismissActivity,
		},
	},
	{
		ID:          RoleDeveloper,
		Name:        "Пользователь",
		Description: "Смотрит свои задачи, отмечает выполнение и при необходимости пишет комментарии.",
		Permissions: []string{
			PermEditOwnTask,
			PermCompleteOwnTask,
			PermCommentTasks,
			PermDismissActivity,
		},
	},
}

func rolePolicies() []RolePolicy {
	out := make([]RolePolicy, len(RoleCatalog))
	for i, role := range RoleCatalog {
		out[i] = role
		out[i].Permissions = append([]string(nil), role.Permissions...)
	}
	return out
}

func normalizeRoleID(roleID, roleName string) string {
	roleID = strings.TrimSpace(strings.ToLower(roleID))
	switch roleID {
	case RoleAdmin, RoleManager, RoleDeveloper:
		return roleID
	}

	name := strings.ToLower(strings.TrimSpace(roleName))
	switch {
	case strings.Contains(name, "админ"):
		return RoleAdmin
	case strings.Contains(name, "менедж"), strings.Contains(name, "руковод"):
		return RoleManager
	default:
		return RoleDeveloper
	}
}

func roleName(roleID string) string {
	for _, role := range RoleCatalog {
		if role.ID == roleID {
			return role.Name
		}
	}
	return "Пользователь"
}

func HasPermission(user User, permission string) bool {
	roleID := normalizeRoleID(user.RoleID, user.Role)
	for _, role := range RoleCatalog {
		if role.ID != roleID {
			continue
		}
		for _, item := range role.Permissions {
			if item == permission {
				return true
			}
		}
	}
	return false
}

func (s *Store) ensureUserRolesLocked() {
	for i := range s.data.Users {
		roleID := normalizeRoleID(s.data.Users[i].RoleID, s.data.Users[i].Role)
		s.data.Users[i].RoleID = roleID
		s.data.Users[i].Role = roleName(roleID)
		if s.data.Users[i].Password == "" {
			s.data.Users[i].Password = defaultPassword(roleID)
		}
	}
}

func (s *Store) ensureUserLoginsLocked() {
	seen := map[string]bool{}
	developerIndex := 0
	for i := range s.data.Users {
		if normalizeRoleID(s.data.Users[i].RoleID, s.data.Users[i].Role) == RoleDeveloper {
			developerIndex++
		}
		login := normalizeLogin(s.data.Users[i].Login)
		if login == "" {
			login = defaultLoginForUser(s.data.Users[i], developerIndex)
		}
		base := login
		for suffix := 2; seen[login]; suffix++ {
			login = fmt.Sprintf("%s%d", base, suffix)
		}
		s.data.Users[i].Login = login
		seen[login] = true
	}
}

func defaultLoginForUser(user User, developerIndex int) string {
	switch user.ID {
	case "USR-001":
		return "manager"
	case "USR-002":
		return "worker1"
	case "USR-003":
		return "worker2"
	}
	if login := loginFromEmail(user.Email); login != "" {
		return login
	}
	switch normalizeRoleID(user.RoleID, user.Role) {
	case RoleAdmin:
		return "admin"
	case RoleManager:
		return "manager"
	default:
		if developerIndex < 1 {
			developerIndex = 1
		}
		return fmt.Sprintf("worker%d", developerIndex)
	}
}

func defaultPassword(roleID string) string {
	switch roleID {
	case RoleAdmin:
		return "admin2026"
	case RoleManager:
		return "manager2026"
	default:
		return "user2026"
	}
}
