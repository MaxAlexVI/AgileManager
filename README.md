# Agile Manager

Agile Manager - учебное веб-приложение для управления задачами, спринтами, уведомлениями и аналитикой в небольших Agile-командах. Приложение работает с PostgreSQL и может запускаться как набор микросервисов через Docker Compose.

## Технологический стек

| Слой | Технологии |
| --- | --- |
| Backend | Go, стандартный `net/http`, REST API, Server-Sent Events |
| Frontend | HTML, CSS, JavaScript без сборщика |
| База данных | PostgreSQL |
| Контейнеризация | Docker, Docker Compose |
| Драйвер БД | `github.com/lib/pq` |
| Архитектура | Gateway + доменные микросервисы + общий PostgreSQL |
| Realtime | SSE endpoint `GET /api/events` |

## Архитектура проекта

| Сервис | Порт | Назначение |
| --- | ---: | --- |
| `gateway` | `8080` | Главная точка входа. Отдает frontend, хранит пользовательские сессии, проверяет авторизацию, проксирует REST-запросы в доменные сервисы и рассылает SSE-события об изменениях. |
| `user-service` | `8081` | Управление пользователями, логинами, ролями и правами доступа. |
| `task-service` | `8082` | Создание и изменение задач, перенос карточек по колонкам, комментарии, отметка выполнения работы. |
| `sprint-service` | `8083` | Создание и изменение спринтов, целей, дат, статусов и ретроспектив. |
| `notification-service` | `8084` | Работа с уведомлениями и отметкой уведомлений как прочитанных. |
| `analytics-service` | `8085` | Отчеты и расчет командной загрузки. |
| `postgres` | `5432` внутри Docker, `15433` на хосте | Основное хранилище данных приложения. |
| `db-init` | без постоянного порта | Одноразовый контейнер. Создает таблицы и стартовые данные при первом запуске. |

В Docker наружу публикуются только:

- `http://localhost:8080` - веб-приложение и gateway API.
- `localhost:15433` - PostgreSQL для локальной проверки из клиента БД.

Доменные сервисы доступны внутри Docker-сети по адресам:

- `http://user-service:8081`
- `http://task-service:8082`
- `http://sprint-service:8083`
- `http://notification-service:8084`
- `http://analytics-service:8085`

## Структура проекта

```text
.
├── cmd/
│   ├── agile-manager/          # Gateway: frontend, авторизация, сессии, proxy, SSE
│   ├── user-service/           # Точка входа User Service
│   ├── task-service/           # Точка входа Task Service
│   ├── sprint-service/         # Точка входа Sprint Service
│   ├── notification-service/   # Точка входа Notification Service
│   ├── analytics-service/      # Точка входа Analytics Service
│   └── db-init/                # Инициализация PostgreSQL для Docker
├── internal/
│   ├── gateway/                # HTTP gateway, REST endpoints, сессии, SSE, proxy в сервисы
│   ├── servicehost/            # Общий HTTP host для доменных микросервисов
│   ├── users/                  # Бизнес-логика пользователей и прав
│   ├── tasks/                  # Бизнес-логика задач, комментариев и выполнения
│   ├── sprints/                # Бизнес-логика спринтов
│   ├── notifications/          # Бизнес-логика уведомлений
│   ├── analytics/              # Бизнес-логика отчетов
│   └── shared/                 # Модели, роли, PostgreSQL store, seed-данные, общие helpers
├── web/
│   └── static/                 # HTML, CSS и JavaScript интерфейса
├── Dockerfile                  # Сборка Go-бинарника выбранного сервиса
├── docker-compose.yml          # Запуск PostgreSQL, gateway и всех микросервисов
├── go.mod                      # Go module и зависимости
└── README.md                   # Техническая документация проекта
```

## Как работает система

1. Пользователь открывает `http://localhost:8080`.
2. Gateway отдает страницу входа из `web/static`.
3. Пользователь отправляет логин и пароль на `POST /api/login`.
4. Gateway проверяет учетные данные через общее хранилище PostgreSQL и создает in-memory session token.
5. Frontend сохраняет token и отправляет его в заголовке:

```http
Authorization: Bearer <token>
```

6. Для чтения общего состояния frontend вызывает `GET /api/state`.
7. Для изменений frontend вызывает gateway API. Gateway проверяет session token и проксирует запрос в нужный микросервис.
8. Доменный сервис проверяет права пользователя через `X-Actor-ID`, выполняет бизнес-логику и сохраняет изменения в PostgreSQL.
9. После успешного изменения gateway отправляет всем подключенным вкладкам SSE-событие через `GET /api/events`.
10. Frontend получает SSE-событие и заново загружает актуальное состояние через `GET /api/state`.

Так изменения карточек, задач, спринтов и уведомлений видны другим пользователям почти сразу после действия.

## Роли и доступ

| Роль | Пользователь | Логин | Пароль | Основные права |
| --- | --- | --- | --- | --- |
| Руководитель | Виктор Пелевин | `manager` | `manager2026` | Пользователи, задачи, спринты, аналитика, уведомления. |
| Работник | Мишель Фуко | `worker1` | `user2026` | Просмотр своих задач, комментарии, отметка выполнения своей задачи, уведомления. |
| Работник | Роберт Смит | `worker2` | `user2026` | Просмотр своих задач, комментарии, отметка выполнения своей задачи, уведомления. |

В коде также есть роль `admin`, но в стартовом наборе пользователей она не создается.

## Работа с базой данных

PostgreSQL используется как единое хранилище для всех сервисов. Каждый сервис при старте проверяет схему БД. Если база пустая, `db-init` создает таблицы и демо-данные.

Строка подключения в Docker:

```text
postgres://postgres:1234@postgres:5432/agile_manager?sslmode=disable
```

На хосте PostgreSQL из Docker доступен так:

```text
postgres://postgres:1234@localhost:15433/agile_manager?sslmode=disable
```

### Таблицы

| Таблица | Назначение | Основные поля |
| --- | --- | --- |
| `users` | Пользователи, логины, пароли и роли. | `id`, `login`, `name`, `role_id`, `role`, `email`, `password` |
| `tasks` | Карточки задач на Kanban-доске. | `id`, `title`, `description`, `status`, `priority`, `assignee_id`, `reporter_id`, `due_date`, `story_points`, `sprint_id`, `created_at`, `updated_at`, `completed_at`, `work_done`, `work_done_at` |
| `comments` | Комментарии к задачам. | `id`, `task_id`, `author_id`, `text`, `created_at` |
| `sprints` | Спринты и их планирование. | `id`, `name`, `goal`, `start_date`, `end_date`, `status`, `retrospective` |
| `notifications` | Персональные уведомления пользователей. | `id`, `kind`, `message`, `task_id`, `user_id`, `is_read`, `created_at` |
| `counters` | Счетчики для генерации человекочитаемых ID. | `key`, `value` |

### Поведение хранилища

- ID создаются в формате `USR-001`, `TASK-001`, `SPR-001`, `CMT-001`, `NTF-001`.
- Уведомления создаются персонально для пользователей, чтобы каждый мог закрывать их независимо.
- `GET /api/state` возвращает уведомления только текущего пользователя.
- Для аналитики данные рассчитываются на основе текущих задач, спринтов и уведомлений.
- При `docker compose down` данные PostgreSQL сохраняются в Docker volume `postgres-data`.
- При `docker compose down -v` volume удаляется, и следующий запуск снова создаст стартовые данные.

## REST API

Все защищенные endpoints требуют заголовок:

```http
Authorization: Bearer <token>
```

При прямом обращении к доменному микросервису внутри Docker-сети вместо session token используется технический заголовок:

```http
X-Actor-ID: USR-001
```

В обычной работе frontend обращается только к gateway на `localhost:8080`.

| Метод | Путь | Описание | Доступ |
| --- | --- | --- | --- |
| `GET` | `/api/health` | Проверка доступности gateway или конкретного сервиса. | Без авторизации. |
| `POST` | `/api/login` | Вход по логину и паролю, выдача session token. | Без авторизации. |
| `POST` | `/api/logout` | Завершение текущей сессии. | Авторизованный пользователь. |
| `GET` | `/api/state` | Полное состояние приложения: колонки, роли, пользователи, задачи, спринты, уведомления текущего пользователя и аналитика. | Авторизованный пользователь. |
| `GET` | `/api/events` | SSE-подключение для realtime-обновлений. Token передается в query: `/api/events?token=...`. | Авторизованный пользователь. |
| `POST` | `/api/users` | Создать пользователя. | `manager` или `admin`, право `manage_users`. |
| `PUT` | `/api/users/{id}` | Обновить пользователя. | `manager` или `admin`, право `manage_users`. |
| `POST` | `/api/tasks` | Создать задачу. | `manager` или `admin`, право `create_tasks`. |
| `PATCH` | `/api/tasks/{id}` | Обновить задачу или перенести карточку в другую колонку. | `manager` или `admin`, право `edit_any_task`. |
| `POST` | `/api/tasks/{id}/complete` | Отметить работу по задаче выполненной и, при необходимости, оставить комментарий. | `manager`/`admin` для любой задачи или работник для своей задачи, право `complete_own_task`. |
| `POST` | `/api/tasks/{id}/comments` | Добавить комментарий к задаче. | Любой авторизованный пользователь с правом `comment_tasks`. |
| `POST` | `/api/sprints` | Создать спринт. | `manager` или `admin`, право `manage_sprints`. |
| `PUT` | `/api/sprints/{id}` | Обновить спринт. | `manager` или `admin`, право `manage_sprints`. |
| `PATCH` | `/api/notifications/{id}/read` | Отметить уведомление прочитанным. | Авторизованный пользователь с правом `dismiss_activity`. |
| `GET` | `/api/reports/team-load` | Получить отчет по загрузке команды. | `manager` или `admin`, право `view_analytics` или `manage_sprints`. |

### Примеры тела запросов

`POST /api/login`

```json
{
  "login": "manager",
  "password": "manager2026"
}
```

`POST /api/tasks`

```json
{
  "title": "Подготовить демо",
  "description": "Собрать сценарий показа доски",
  "status": "backlog",
  "priority": "medium",
  "assigneeId": "USR-002",
  "reporterId": "USR-001",
  "dueDate": "2026-06-01",
  "storyPoints": 3,
  "sprintId": "SPR-001"
}
```

`PATCH /api/tasks/{id}`

```json
{
  "status": "review"
}
```

`POST /api/tasks/{id}/complete`

```json
{
  "comment": "Работа готова к проверке"
}
```

## Быстрый старт через Docker

Перед запуском проверьте, что Docker Desktop запущен.

1. Перейти в папку проекта:

```powershell
cd C:\Users\Admin\Documents\Agile-manager
```

2. Собрать образы:

```powershell
docker compose build
```

3. Запустить приложение в фоне:

```powershell
docker compose up -d
```

4. Проверить контейнеры:

```powershell
docker compose ps
```

5. Проверить gateway:

```powershell
Invoke-RestMethod http://localhost:8080/api/health
```

Ожидаемый ответ:

```json
{"status":"ok"}
```

6. Открыть приложение:

```text
http://localhost:8080
```

### Полезные Docker команды

Посмотреть логи всех сервисов:

```powershell
docker compose logs -f
```

Посмотреть логи конкретного сервиса:

```powershell
docker compose logs -f gateway
docker compose logs -f task-service
docker compose logs -f notification-service
```

Остановить контейнеры без удаления данных:

```powershell
docker compose down
```

Остановить контейнеры и удалить данные PostgreSQL:

```powershell
docker compose down -v
```

Пересобрать и перезапустить только gateway после изменения frontend:

```powershell
docker compose build gateway
docker compose up -d gateway
```

## Локальный запуск без Docker

Для локального запуска нужен доступный PostgreSQL и переменная `DATABASE_URL`.

### Один процесс

```powershell
$env:DATABASE_URL="postgres://postgres:1234@localhost:5433/agile_manager?sslmode=disable"
go run ./cmd/agile-manager
```

После запуска:

```text
http://localhost:8080
```

### Микросервисы вручную

В отдельных терминалах:

```powershell
$env:DATABASE_URL="postgres://postgres:1234@localhost:5433/agile_manager?sslmode=disable"
go run ./cmd/user-service -addr :8081
go run ./cmd/task-service -addr :8082
go run ./cmd/sprint-service -addr :8083
go run ./cmd/notification-service -addr :8084
go run ./cmd/analytics-service -addr :8085
```

Затем запустить gateway:

```powershell
$env:DATABASE_URL="postgres://postgres:1234@localhost:5433/agile_manager?sslmode=disable"
$env:USER_SERVICE_URL="http://localhost:8081"
$env:TASK_SERVICE_URL="http://localhost:8082"
$env:SPRINT_SERVICE_URL="http://localhost:8083"
$env:NOTIFICATION_SERVICE_URL="http://localhost:8084"
$env:ANALYTICS_SERVICE_URL="http://localhost:8085"
go run ./cmd/agile-manager
```

## Проверка

Запуск тестов:

```powershell
go test ./...
```

Проверка Docker-конфигурации:

```powershell
docker compose config
```
