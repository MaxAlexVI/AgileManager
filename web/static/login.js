const el = (id) => document.getElementById(id);

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json();
}

function redirectAuthorizedUser() {
  if (localStorage.getItem("agile.authToken")) {
    window.location.href = "/app.html";
  }
}

async function login(event) {
  event.preventDefault();
  try {
    const result = await api("/api/login", {
      method: "POST",
      body: JSON.stringify({
        login: el("loginLogin").value.trim(),
        password: el("loginPassword").value,
      }),
    });
    localStorage.setItem("agile.authToken", result.token);
    localStorage.setItem("agile.currentUser", JSON.stringify(result.user));
    window.location.href = "/app.html";
  } catch (error) {
    showToast("Не удалось войти", error.message);
  }
}

function showToast(title, message) {
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
  el("toastStack").prepend(toast);
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

el("loginForm").addEventListener("submit", login);
redirectAuthorizedUser();
