const state = {
  data: null,
  selectedTaskId: null,
  token: localStorage.getItem("agile.authToken") || "",
  currentUser: JSON.parse(localStorage.getItem("agile.currentUser") || "null"),
  loadedOnce: false,
  seenNotificationIds: new Set(),
  refreshInFlight: false,
  refreshTimer: null,
  eventSource: null,
};

const el = (id) => document.getElementById(id);
const formatDate = (value) => value ? new Date(value).toLocaleDateString("ru-RU") : "";
const priorityTitle = { low: "Низкий", medium: "Средний", high: "Высокий", critical: "Критический" };
const sprintStatusTitle = { planned: "Запланирован", active: "Активен", closed: "Закрыт" };
async function api(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
  if (state.token) {
    headers.Authorization = `Bearer ${state.token}`;
  }
  const response = await fetch(path, { headers, ...options });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json();
}

async function logout() {
  if (state.token) {
    await api("/api/logout", { method: "POST", body: "{}" }).catch(() => {});
  }
  state.token = "";
  state.currentUser = null;
  state.data = null;
  state.loadedOnce = false;
  state.seenNotificationIds.clear();
  if (state.eventSource) {
    state.eventSource.close();
    state.eventSource = null;
  }
  window.clearInterval(state.refreshTimer);
  localStorage.removeItem("agile.authToken");
  localStorage.removeItem("agile.currentUser");
  window.location.href = "/";
}

async function loadState(options = {}) {
  if (!state.token) {
    window.location.href = "/";
    return;
  }
  if (state.refreshInFlight) {
    return;
  }
  state.refreshInFlight = true;
  try {
    const nextData = await api("/api/state");
    trackNewNotifications(nextData.notifications || []);
    state.data = nextData;
    state.currentUser = userById(state.currentUser?.id) || state.currentUser;
    if (!state.selectedTaskId && state.data.tasks.length) {
      state.selectedTaskId = state.data.tasks[0].id;
    }
    state.loadedOnce = true;
    renderAll();
  } catch (error) {
    if (options.silent) {
      console.warn("Не удалось обновить состояние", error);
      return;
    }
    clearSessionAndRedirect();
  } finally {
    state.refreshInFlight = false;
  }
}

function clearSessionAndRedirect() {
  localStorage.removeItem("agile.authToken");
  localStorage.removeItem("agile.currentUser");
  window.location.href = "/";
}

function startStatePolling() {
  window.clearInterval(state.refreshTimer);
  state.refreshTimer = window.setInterval(() => {
    if (!document.hidden) {
      loadState({ silent: true });
    }
  }, 30000);
}

function connectRealtime() {
  if (!window.EventSource || !state.token) {
    return;
  }
  if (state.eventSource) {
    state.eventSource.close();
  }

  state.eventSource = new EventSource(`/api/events?token=${encodeURIComponent(state.token)}`);
  state.eventSource.addEventListener("state", () => {
    loadState({ silent: true });
  });
  state.eventSource.onerror = () => {
    console.warn("Realtime connection interrupted; polling fallback is still active.");
  };
}

function userById(id) {
  return state.data?.users.find((user) => user.id === id);
}

function currentRole() {
  const user = userById(state.currentUser?.id) || state.currentUser;
  return state.data?.roles.find((role) => role.id === user?.roleId) || state.data?.roles[0];
}

function can(permission) {
  return currentRole()?.permissions.includes(permission) || false;
}

function canEditTask(task) {
  if (!task) return false;
  return can("edit_any_task");
}

function canMoveTask(task) {
  if (!task) return false;
  return can("edit_any_task");
}

function canCompleteTask(task) {
  if (!task) return false;
  return can("edit_any_task") || (can("complete_own_task") && task.assigneeId === state.currentUser?.id);
}

function userName(id) {
  return userById(id)?.name || "Без исполнителя";
}

function sprintName(id) {
  return state.data?.sprints.find((sprint) => sprint.id === id)?.name || "Без спринта";
}

function trackNewNotifications(notifications) {
  if (!state.loadedOnce) {
    notifications.forEach((note) => state.seenNotificationIds.add(note.id));
    return;
  }
  notifications
    .filter((note) => !note.read && !state.seenNotificationIds.has(note.id))
    .reverse()
    .forEach((note) => {
      state.seenNotificationIds.add(note.id);
      showToast("Новое уведомление", note.message);
    });
}

function renderAll() {
  fillSelects();
  renderBoard();
  renderReviewQueue();
  renderActivity();
  renderSprints();
  renderSprintDetailsActions();
  renderAnalytics();
  renderUsers();
  applyRoleControls();
}

function fillSelects() {
  const userOptions = ['<option value="">Все</option>', ...state.data.users.map((user) => `<option value="${user.id}">${escapeHtml(user.name)}</option>`)].join("");
  const sprintOptions = ['<option value="">Все</option>', ...state.data.sprints.map((sprint) => `<option value="${sprint.id}">${escapeHtml(sprint.name)}</option>`)].join("");
  el("assigneeFilter").innerHTML = userOptions;
  el("sprintFilter").innerHTML = sprintOptions;
  el("taskAssignee").innerHTML = state.data.users.map((user) => `<option value="${user.id}">${escapeHtml(user.name)} · ${escapeHtml(user.role)}</option>`).join("");
  el("taskSprint").innerHTML = ['<option value="">Без спринта</option>', ...state.data.sprints.map((sprint) => `<option value="${sprint.id}">${escapeHtml(sprint.name)}</option>`)].join("");
  el("taskStatus").innerHTML = state.data.columns.map((column) => `<option value="${column.id}">${escapeHtml(column.title)}</option>`).join("");
  el("userRole").innerHTML = state.data.roles.map((role) => `<option value="${role.id}">${escapeHtml(role.name)}</option>`).join("");
}

function renderSession() {
  const user = userById(state.currentUser?.id) || state.currentUser;
  el("sessionUser").textContent = `${user?.name || "Пользователь"} · ${user?.role || ""}`;
}

function applyRoleControls() {
  renderSession();
  el("openTaskForm").disabled = !can("create_tasks");
  document.querySelectorAll(".manager-tab").forEach((tab) => tab.hidden = !can("edit_any_task"));
  document.querySelectorAll(".users-tab").forEach((tab) => tab.hidden = !can("manage_users"));
  if (!can("edit_any_task") && el("reviewPanel").classList.contains("active")) {
    activateTab("board");
  }
  if (!can("manage_users") && el("usersPanel").classList.contains("active")) {
    activateTab("board");
  }
  const sprintAllowed = can("manage_sprints");
  el("sprintForm").hidden = !sprintAllowed;
  el("saveSprintButton").disabled = !sprintAllowed;
  document.querySelectorAll("#sprintForm input, #sprintForm textarea, #sprintForm select").forEach((field) => {
    field.disabled = !sprintAllowed;
  });
}

function filteredTasks() {
  const query = el("taskSearch").value.trim().toLowerCase();
  const assignee = el("assigneeFilter").value;
  const sprint = el("sprintFilter").value;
  const mineOnly = el("myTasksOnly").checked;
  return state.data.tasks.filter((task) => {
    const text = `${task.title} ${task.description} ${task.priority}`.toLowerCase();
    return (!query || text.includes(query)) &&
      (!assignee || task.assigneeId === assignee) &&
      (!sprint || task.sprintId === sprint) &&
      (!mineOnly || task.assigneeId === state.currentUser?.id);
  });
}

function renderBoard() {
  const tasks = filteredTasks();
  el("board").innerHTML = state.data.columns.map((column) => {
    const columnTasks = tasks.filter((task) => task.status === column.id);
    return `
      <section class="column" data-status="${column.id}">
        <div class="column-head">
          <h2>${escapeHtml(column.title)}</h2>
          <span class="count">${columnTasks.length}</span>
        </div>
        <div class="column-body">
          ${columnTasks.map(renderTaskCard).join("") || '<p class="muted">Нет задач</p>'}
        </div>
      </section>`;
  }).join("");

  document.querySelectorAll(".task-card").forEach((card) => {
    card.addEventListener("click", () => {
      state.selectedTaskId = card.dataset.id;
      renderBoard();
    });
    card.addEventListener("dblclick", () => {
      const task = state.data.tasks.find((item) => item.id === card.dataset.id);
      openTaskDialog(task);
    });
    card.addEventListener("dragstart", (event) => {
      const task = state.data.tasks.find((item) => item.id === card.dataset.id);
      if (!canMoveTask(task)) {
        event.preventDefault();
        showToast("Недостаточно прав", "Эта роль не может перемещать выбранную задачу.");
        return;
      }
      event.dataTransfer.setData("text/plain", card.dataset.id);
    });
  });

  document.querySelectorAll("[data-complete-task]").forEach((button) => {
    button.addEventListener("click", (event) => {
      event.stopPropagation();
      const task = state.data.tasks.find((item) => item.id === button.dataset.completeTask);
      openCompleteDialog(task);
    });
  });

  document.querySelectorAll(".column").forEach((column) => {
    column.addEventListener("dragover", (event) => {
      event.preventDefault();
      column.classList.add("drag-over");
    });
    column.addEventListener("dragleave", () => column.classList.remove("drag-over"));
    column.addEventListener("drop", async (event) => {
      event.preventDefault();
      column.classList.remove("drag-over");
      const taskId = event.dataTransfer.getData("text/plain");
      const task = state.data.tasks.find((item) => item.id === taskId);
      if (!canMoveTask(task)) {
        showToast("Недостаточно прав", "Эта роль не может перемещать выбранную задачу.");
        return;
      }
      await updateTask(taskId, { status: column.dataset.status });
    });
  });
}

function renderTaskCard(task) {
  const selected = task.id === state.selectedTaskId ? " selected" : "";
  const completeAllowed = canCompleteTask(task) && !task.workDone;
  const draggable = canMoveTask(task) ? "true" : "false";
  return `
    <article class="task-card${selected}" draggable="${draggable}" data-id="${task.id}">
      <h3>${escapeHtml(task.title)}</h3>
      <p class="task-description">${escapeHtml(task.description || "Без описания")}</p>
      <div class="meta">
        <span class="badge ${task.priority}">${priorityTitle[task.priority] || task.priority}</span>
        <span class="badge">${task.storyPoints} SP</span>
        <span class="badge">${escapeHtml(userName(task.assigneeId))}</span>
        ${task.workDone ? '<span class="badge done-stage">Выполнено на этапе</span>' : ""}
      </div>
      <div class="small">${escapeHtml(sprintName(task.sprintId))}${task.dueDate ? ` · до ${task.dueDate}` : ""}</div>
      <div class="card-actions">
        <button class="secondary-button compact-button" type="button" data-complete-task="${task.id}" ${completeAllowed ? "" : "disabled"}>✓ Выполнено</button>
      </div>
    </article>`;
}

function renderReviewQueue() {
  const items = state.data.tasks
    .filter((task) => task.workDone && task.status !== "done")
    .sort((a, b) => new Date(b.workDoneAt) - new Date(a.workDoneAt));
  el("reviewCount").textContent = items.length;
  el("reviewList").innerHTML = items.map((task) => {
    const next = nextStatus(task.status);
    const comment = reviewComment(task);
    return `
      <article class="review-item">
        <div>
          <h3>${escapeHtml(task.title)}</h3>
          <p class="review-description">${escapeHtml(task.description || "Без описания")}</p>
          <div class="meta">
            <span class="badge">${escapeHtml(columnName(task.status))}</span>
            <span class="badge">${escapeHtml(userName(task.assigneeId))}</span>
            <span class="badge done-stage">Отмечено выполненным</span>
          </div>
          <div class="review-comment">
            <span>Комментарий исполнителя</span>
            <p>${escapeHtml(comment || "Комментарий не оставлен")}</p>
          </div>
        </div>
        <div class="review-actions">
          ${next ? `<button class="primary-button compact-button" type="button" data-advance-task="${task.id}">Дальше: ${escapeHtml(columnName(next))}</button>` : ""}
        </div>
      </article>`;
  }).join("") || '<p class="muted">Нет задач, ожидающих проверки.</p>';

  document.querySelectorAll("[data-advance-task]").forEach((button) => {
    button.addEventListener("click", async () => {
      const task = state.data.tasks.find((item) => item.id === button.dataset.advanceTask);
      const next = nextStatus(task?.status);
      if (task && next) {
        await updateTask(task.id, { status: next });
      }
    });
  });
}

function reviewComment(task) {
  const comments = (task.comments || [])
    .filter((comment) => comment.authorId === task.assigneeId)
    .sort((a, b) => new Date(b.createdAt) - new Date(a.createdAt));
  return comments[0]?.text || "";
}

function columnName(status) {
  return state.data?.columns.find((column) => column.id === status)?.title || status;
}

function nextStatus(status) {
  const index = state.data.columns.findIndex((column) => column.id === status);
  return index >= 0 && index < state.data.columns.length - 1 ? state.data.columns[index + 1].id : "";
}

function renderActivity() {
  const activity = state.data.analytics.recentActivity || [];
  el("activityList").innerHTML = activity.map((note) => `
    <div class="activity-item" data-notification-id="${note.id}">
      <div>
        <strong>${escapeHtml(note.message)}</strong>
        <time>${formatDate(note.createdAt)}</time>
      </div>
      <button class="activity-dismiss" type="button" title="Закрыть уведомление" aria-label="Закрыть уведомление" data-dismiss-notification="${note.id}" ${can("dismiss_activity") ? "" : "disabled"}>×</button>
    </div>`).join("") || '<p class="muted">Уведомлений нет</p>';

  document.querySelectorAll("[data-dismiss-notification]").forEach((button) => {
    button.addEventListener("click", () => dismissNotification(button.dataset.dismissNotification));
  });
}

async function dismissNotification(id) {
  state.data.analytics.recentActivity = (state.data.analytics.recentActivity || []).filter((note) => note.id !== id);
  state.data.notifications = (state.data.notifications || []).map((note) => note.id === id ? { ...note, read: true } : note);
  renderActivity();
  await runAction(() => api(`/api/notifications/${id}/read`, { method: "PATCH", body: "{}" }));
  await loadState();
}

function renderSprints() {
  const canManage = can("manage_sprints");
  el("sprintList").innerHTML = state.data.sprints.map((sprint) => {
    const progress = state.data.analytics.sprintProgress.find((item) => item.sprintId === sprint.id);
    const percent = Math.round((progress?.completionRatio || 0) * 100);
    return `
      <article class="sprint-item">
        <h2>${escapeHtml(sprint.name)}</h2>
        <p>${escapeHtml(sprint.goal || "Без цели")}</p>
        <div class="meta">
          <span class="badge">${sprintStatusTitle[sprint.status] || sprint.status}</span>
          <span class="badge">${sprint.startDate || "?"} - ${sprint.endDate || "?"}</span>
          <span class="badge">${progress?.doneTasks || 0}/${progress?.totalTasks || 0}</span>
        </div>
        <div class="progress" style="--value:${percent}%"><i></i></div>
        ${sprint.retrospective ? `<p class="small">${escapeHtml(sprint.retrospective)}</p>` : ""}
        ${canManage ? `<button class="secondary-button" data-edit-sprint="${sprint.id}">Редактировать</button>` : ""}
      </article>`;
  }).join("");

  document.querySelectorAll("[data-edit-sprint]").forEach((button) => {
    button.addEventListener("click", () => {
      const sprint = state.data.sprints.find((item) => item.id === button.dataset.editSprint);
      el("sprintId").value = sprint.id;
      el("sprintName").value = sprint.name;
      el("sprintGoal").value = sprint.goal;
      el("sprintStart").value = sprint.startDate;
      el("sprintEnd").value = sprint.endDate;
      el("sprintStatus").value = sprint.status;
      el("sprintRetro").value = sprint.retrospective;
    });
  });
}

function renderSprintDetailsActions() {
  document.querySelectorAll(".sprint-item").forEach((item, index) => {
    const sprint = state.data.sprints[index];
    if (!sprint || item.querySelector("[data-open-sprint]")) {
      return;
    }
    const actions = document.createElement("div");
    actions.className = "sprint-actions";
    actions.innerHTML = `<button class="secondary-button" type="button" data-open-sprint="${sprint.id}">Открыть</button>`;
    item.append(actions);
  });
  document.querySelectorAll("[data-open-sprint]").forEach((button) => {
    button.addEventListener("click", () => {
      const sprint = state.data.sprints.find((item) => item.id === button.dataset.openSprint);
      openSprintDialog(sprint);
    });
  });
}

function openSprintDialog(sprint) {
  if (!sprint) return;
  const tasks = state.data.tasks.filter((task) => task.sprintId === sprint.id);
  const progress = state.data.analytics.sprintProgress.find((item) => item.sprintId === sprint.id);
  const percent = Math.round((progress?.completionRatio || 0) * 100);
  el("sprintDialogTitle").textContent = sprint.name;
  el("sprintDialogMeta").innerHTML = `
    <span class="badge">${sprintStatusTitle[sprint.status] || sprint.status}</span>
    <span class="badge">${sprint.startDate || "?"} - ${sprint.endDate || "?"}</span>
    <span class="badge">${progress?.doneTasks || 0}/${progress?.totalTasks || 0}</span>
    <span class="badge">${percent}%</span>`;
  el("sprintDialogGoal").textContent = sprint.goal || "Без цели";
  el("sprintTaskList").innerHTML = tasks.map((task) => `
    <article class="sprint-task-row">
      <div>
        <strong>${escapeHtml(task.title)}</strong>
        <p class="small">${escapeHtml(userName(task.assigneeId))} · ${escapeHtml(columnName(task.status))}${task.workDone ? " · отмечено выполненным" : ""}</p>
      </div>
      <button class="secondary-button compact-button" type="button" data-open-task-from-sprint="${task.id}">Открыть</button>
    </article>`).join("") || '<p class="muted">В этом спринте пока нет задач.</p>';
  document.querySelectorAll("[data-open-task-from-sprint]").forEach((button) => {
    button.addEventListener("click", () => {
      const task = state.data.tasks.find((item) => item.id === button.dataset.openTaskFromSprint);
      el("sprintDialog").close();
      openTaskDialog(task);
    });
  });
  el("sprintDialog").showModal();
}

function renderAnalytics() {
  const analytics = state.data.analytics;
  const teamLoad = analytics.teamLoad || [];
  const dueSoonTasks = analytics.dueSoonTasks || analytics.blockedSoonDueTask || [];
  el("metrics").innerHTML = `
    <div class="metric"><span>Активные задачи</span><strong>${analytics.activeTasks}</strong></div>
    <div class="metric"><span>Завершено</span><strong>${analytics.completedTasks}</strong></div>
    <div class="metric"><span>Velocity</span><strong>${analytics.velocityPoints}</strong></div>
    <div class="metric"><span>WIP</span><strong>${analytics.workInProgress}</strong></div>`;

  el("teamLoad").innerHTML = teamLoad.map((member) => `
    <div class="load-row">
      <strong>${escapeHtml(member.name)}</strong>
      <p class="small">${escapeHtml(member.role)} · активных: ${member.activeTasks} · done: ${member.doneTasks} · ${member.storyPoints} SP</p>
    </div>`).join("");

  el("dueSoon").innerHTML = dueSoonTasks.map((task) => `
    <div class="due-row">
      <strong>${escapeHtml(task.title)}</strong>
      <p class="small">${escapeHtml(task.assignee || "Без исполнителя")} · ${task.dueDate}</p>
    </div>`).join("") || '<p class="muted">Нет срочных задач</p>';
}

function renderUsers() {
  el("userList").innerHTML = state.data.users.map((user) => `
    <article class="user-row">
      <div>
        <strong>${escapeHtml(user.name)}</strong>
        <p class="small">${escapeHtml(user.login)} · ${escapeHtml(user.role)}${user.email ? ` · ${escapeHtml(user.email)}` : ""}</p>
      </div>
      <button class="secondary-button compact-button" type="button" data-edit-user="${user.id}">Редактировать</button>
    </article>`).join("");

  document.querySelectorAll("[data-edit-user]").forEach((button) => {
    button.addEventListener("click", () => {
      const user = state.data.users.find((item) => item.id === button.dataset.editUser);
      fillUserForm(user);
    });
  });
}

function fillUserForm(user = null) {
  el("userId").value = user?.id || "";
  el("userLogin").value = user?.login || "";
  el("userName").value = user?.name || "";
  el("userEmail").value = user?.email || "";
  el("userRole").value = user?.roleId || "developer";
  el("userPassword").value = "";
  el("userPassword").placeholder = user ? "Оставьте пустым, чтобы не менять" : "Если пусто, будет демо-пароль роли";
}

async function saveUser(event) {
  event.preventDefault();
  const id = el("userId").value;
  const payload = {
    login: el("userLogin").value.trim(),
    name: el("userName").value.trim(),
    email: el("userEmail").value.trim(),
    roleId: el("userRole").value,
    password: el("userPassword").value.trim(),
  };
  await runAction(() => api(id ? `/api/users/${id}` : "/api/users", {
    method: id ? "PUT" : "POST",
    body: JSON.stringify(payload),
  }));
  fillUserForm();
  await loadState();
}

function openTaskDialog(task = null) {
  if (!task && !can("create_tasks")) {
    showToast("Недостаточно прав", "Только администратор или руководитель может создавать задачи.");
    return;
  }

  el("taskFormTitle").textContent = task ? (canEditTask(task) ? "Редактирование задачи" : "Просмотр задачи") : "Новая задача";
  el("taskId").value = task?.id || "";
  el("taskTitle").value = task?.title || "";
  el("taskDescription").value = task?.description || "";
  el("taskStatus").value = task?.status || "backlog";
  el("taskPriority").value = task?.priority || "medium";
  el("taskAssignee").value = task?.assigneeId || state.data.users[0]?.id || "";
  el("taskPoints").value = task?.storyPoints ?? 3;
  el("taskDue").value = task?.dueDate || "";
  el("taskSprint").value = task?.sprintId || "";
  applyTaskFormPermissions(task);
  applyTaskDialogMode(task);
  el("taskDialog").showModal();
}

function applyTaskFormPermissions(task) {
  const editAny = can("edit_any_task") || (!task && can("create_tasks"));
  const allowed = editAny;
  el("saveTaskButton").disabled = !allowed;
  ["taskTitle", "taskPriority", "taskAssignee", "taskPoints", "taskSprint"].forEach((id) => {
    el(id).disabled = !editAny;
  });
  ["taskDescription", "taskStatus", "taskDue"].forEach((id) => {
    el(id).disabled = !allowed;
  });
}

function applyTaskDialogMode(task) {
  const readonly = task && !canEditTask(task);
  el("taskReadonlyContent").hidden = !readonly;
  document.querySelector(".task-content-section").hidden = !!readonly;
  document.querySelector(".task-planning-section").hidden = !!readonly;
  document.querySelector("#taskForm menu").hidden = readonly;
  if (readonly) {
    el("readonlyTaskTitle").textContent = task.title || "Без названия";
    el("readonlyTaskDescription").textContent = task.description || "Без описания";
  }
}

function openCompleteDialog(task) {
  if (!task || !canCompleteTask(task)) {
    showToast("Недостаточно прав", "Эта роль не может отмечать выполнение выбранной задачи.");
    return;
  }
  el("completeTaskId").value = task.id;
  el("completeTaskTitle").textContent = `Выполнение: ${task.title}`;
  el("completeComment").value = "";
  el("completeDialog").showModal();
}

async function saveTask(event) {
  event.preventDefault();
  const payload = {
    title: el("taskTitle").value.trim(),
    description: el("taskDescription").value.trim(),
    status: el("taskStatus").value,
    priority: el("taskPriority").value,
    assigneeId: el("taskAssignee").value,
    reporterId: state.currentUser?.id || "",
    dueDate: el("taskDue").value,
    storyPoints: Number(el("taskPoints").value || 0),
    sprintId: el("taskSprint").value,
  };
  const id = el("taskId").value;
  if (id) {
    await updateTask(id, payload);
  } else {
    const created = await runAction(() => api("/api/tasks", { method: "POST", body: JSON.stringify(payload) }));
    if (created?.id) state.selectedTaskId = created.id;
    await loadState();
  }
  el("taskDialog").close();
}

async function completeTask(event) {
  event.preventDefault();
  const taskId = el("completeTaskId").value;
  const comment = el("completeComment").value.trim();
  await runAction(() => api(`/api/tasks/${taskId}/complete`, {
    method: "POST",
    body: JSON.stringify({ comment }),
  }));
  state.selectedTaskId = taskId;
  el("completeDialog").close();
  await loadState();
}

async function updateTask(id, patch) {
  await runAction(() => api(`/api/tasks/${id}`, { method: "PATCH", body: JSON.stringify(patch) }));
  state.selectedTaskId = id;
  await loadState();
}

async function runAction(action) {
  try {
    return await action();
  } catch (error) {
    showToast("Действие не выполнено", error.message);
    throw error;
  }
}

function showToast(title, message) {
  const stack = el("toastStack");
  if (!stack) return;
  const toast = document.createElement("div");
  toast.className = "toast";
  toast.innerHTML = `
    <div>
      <strong>${escapeHtml(title)}</strong>
      <p>${escapeHtml(message)}</p>
    </div>
    <button class="toast-close" type="button" title="Закрыть" aria-label="Закрыть">×</button>`;

  const close = () => {
    toast.remove();
    window.clearTimeout(timer);
  };
  const timer = window.setTimeout(close, 4200);
  toast.querySelector(".toast-close").addEventListener("click", close);
  stack.prepend(toast);
}

function activateTab(tabName) {
  document.querySelectorAll(".tab").forEach((item) => item.classList.toggle("active", item.dataset.tab === tabName));
  document.querySelectorAll(".tab-panel").forEach((panel) => panel.classList.remove("active"));
  el(`${tabName}Panel`).classList.add("active");
}

function initEvents() {
  el("logoutButton").addEventListener("click", logout);
  el("openTaskForm").addEventListener("click", () => openTaskDialog());
  el("closeTaskDialog").addEventListener("click", () => el("taskDialog").close());
  el("cancelTaskDialog").addEventListener("click", () => el("taskDialog").close());
  el("closeCompleteDialog").addEventListener("click", () => el("completeDialog").close());
  el("cancelCompleteDialog").addEventListener("click", () => el("completeDialog").close());
  el("closeSprintDialog").addEventListener("click", () => el("sprintDialog").close());
  el("taskForm").addEventListener("submit", saveTask);
  el("completeForm").addEventListener("submit", completeTask);
  el("userForm").addEventListener("submit", saveUser);
  el("resetUserForm").addEventListener("click", () => fillUserForm());
  el("taskSearch").addEventListener("input", renderBoard);
  el("assigneeFilter").addEventListener("change", renderBoard);
  el("sprintFilter").addEventListener("change", renderBoard);
  el("myTasksOnly").addEventListener("change", renderBoard);

  document.querySelectorAll(".tab").forEach((tab) => {
    tab.addEventListener("click", () => activateTab(tab.dataset.tab));
  });

  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) {
      loadState({ silent: true });
    }
  });

  el("sprintForm").addEventListener("submit", async (event) => {
    event.preventDefault();
    const id = el("sprintId").value;
    const payload = {
      name: el("sprintName").value.trim(),
      goal: el("sprintGoal").value.trim(),
      startDate: el("sprintStart").value,
      endDate: el("sprintEnd").value,
      status: el("sprintStatus").value,
      retrospective: el("sprintRetro").value.trim(),
    };
    await runAction(() => api(id ? `/api/sprints/${id}` : "/api/sprints", {
      method: id ? "PUT" : "POST",
      body: JSON.stringify(payload),
    }));
    event.target.reset();
    el("sprintId").value = "";
    await loadState();
  });
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

initEvents();
loadState().then(() => {
  connectRealtime();
  startStatePolling();
}).catch((error) => {
  document.body.innerHTML = `<main class="shell"><section class="detail roomy"><h1>Ошибка запуска</h1><p>${escapeHtml(error.message)}</p></section></main>`;
});
