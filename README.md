# go-clockify

A Clockify-style time tracker: a Go API (standard `net/http` + GORM + Postgres) with a Next.js frontend, all running in Docker.

## Run it

```bash
cp .env.example .env        # first time only
docker compose up -d --build
```

| Service | URL | Notes |
|---|---|---|
| Frontend (Next.js) | http://localhost:3001 | hot reload |
| Backend (Go) | http://localhost:8090/health | rebuilds and restarts on every `.go` save (air) |
| Postgres | `localhost:5433` | user/password/db: `clockify` |

```bash
docker compose logs -f backend     # watch the Go server (build errors show up here)
docker compose logs -f frontend
docker compose exec db psql -U clockify -d clockify   # SQL shell
docker compose down                # stop
docker compose down -v             # stop and delete the database
```

## Layout

```
backend/
  cmd/server/main.go        entry point: config → database → routes → listen; graceful shutdown
  cmd/server/cors.go        lets the frontend (port 3001) call the API from the browser
  models/                   GORM structs = tables (Task, TaskItem, Project) + small helpers (TotalHours, IsRunning)
  controllers/              plain functions: (ctx, input) → (result, error); all database logic lives here
  routes/                   HTTP: read params/body → call a controller → status code + JSON
  docs/walkthrough.md       guided tour: follow one request through every layer
  docs/concurrency.md       where context, WaitGroups and channels are used, and why
  internal/config/          reads env vars (DATABASE_URL is required)
  internal/database/        GORM connection + Migrate()
  internal/sweeper/         background goroutine: stops timers that stop being pinged
  Dockerfile                dev target (air live reload) and prod target (static binary)
frontend/
  src/lib/api.ts            typed fetch calls to the Go API
  src/lib/time.ts           durations, including the live time of a running timer
  src/app/page.tsx          tracker (/) — start/stop timers, manual entries
  src/app/dashboard/page.tsx  dashboard (/dashboard) — running timers + totals
  src/app/projects/page.tsx   projects (/projects) — create, edit, delete
  src/components/           tracker, task-row, new-entry-form, dashboard, projects, project-select
docker-compose.yml          db + backend + frontend
```

**Request flow:** `routes` (HTTP) → `controllers` (logic + GORM) → `models` (tables). Controllers never touch HTTP;
they return errors like `ErrInvalidInput`, and `routes/response.go` turns them into 400 / 404 / 409. Every controller
also takes a `context.Context` from the request and passes it to `db.WithContext(ctx)`, so a query stops when the
client disconnects. See [`backend/docs/concurrency.md`](backend/docs/concurrency.md) for context, WaitGroups and channels.

## API

| Method | Path | Body | Does |
|---|---|---|---|
| GET | `/tasks` | | all tasks with their entries, `project`, `total_hours`, `timer_count`, `running` |
| GET | `/tasks/{id}` | | one task, same shape |
| POST | `/tasks` | `{"title": "…", "project_id"?: 3}` | create (titles are unique → 409 on a duplicate) |
| PATCH | `/tasks/{id}` | `{"title"?: "…", "status"?: "todo" \| "in_progress" \| "done"}` | update |
| PUT | `/tasks/{id}/project` | `{"project_id": 3}` or `{"project_id": null}` | assign the task to a project, or clear it |
| DELETE | `/tasks/{id}` | | delete the task and its entries |
| POST | `/tasks/{id}/start` | | **auto entry**: start a timer now. Several tasks can run at once; a task already running → 409 |
| POST | `/items/{id}/stop` | | stop a running timer → hours = end − start |
| POST | `/tasks/{id}/items` | `{"start_at": "2026-10-12T09:00:00Z", "manual_hours": 1.5}` | **manual entry** |
| PATCH | `/items/{id}` | `{"start_at": "…", "end_at"?: "…", "manual_hours"?: 1.5}` | edit an entry (end_at **or** manual_hours) |
| DELETE | `/items/{id}` | | delete an entry |
| GET | `/projects` | | all projects with `task_count` and `total_hours` |
| GET | `/projects/{id}` | | one project, same shape |
| POST | `/projects` | `{"title": "…", "client_name"?: "…"}` | create (titles are unique → 409 on a duplicate) |
| PATCH | `/projects/{id}` | `{"title"?: "…", "client_name"?: "…"}` | update |
| DELETE | `/projects/{id}` | | delete the project; its tasks are kept with `project_id = NULL` |
| GET | `/dashboard` | | running timers (with task + project) and today / week / all-time totals |
| POST | `/timers/ping` | | heartbeat: the browser calls it every few seconds while a tab is open. A timer that stops being pinged for 2 minutes is stopped automatically (see below) |

`total_hours` leaves out a running timer; the UI adds its live time from `start_at`.

**Auto-stop on a dead tab.** A timer that is started and never stopped would run forever, so the browser sends
`POST /timers/ping` while a tab is open. The server records `last_ping_at` on every running timer, and a background
goroutine (`internal/sweeper`) stops any timer whose last ping is older than `TIMER_PING_TIMEOUT` (default `2m`).
Its `end_at` is set to the **last ping**, not to the moment of the sweep, so closing a tab never books time nobody
was there for.

| Env var | Default | Does |
|---|---|---|
| `PORT` | `8080` | listen port |
| `DATABASE_URL` | *required* | Postgres DSN (no default on purpose: fail fast) |
| `FRONTEND_ORIGIN` | `http://localhost:3001` | the single origin allowed by CORS |
| `TIMER_PING_TIMEOUT` | `2m` | stop a timer that has not been pinged for this long |
| `SWEEP_INTERVAL` | `30s` | how often the sweeper looks; `0` disables it |

Code inside `internal/` can only be imported by this module. That's Go's way of keeping packages private.

## Adding code in Go

**1. A model** (a struct = a table), e.g. `backend/models/project.model.go`:

```go
package models

import "gorm.io/gorm"

type Project struct {
	gorm.Model        // adds ID, CreatedAt, UpdatedAt, DeletedAt
	Name       string `gorm:"not null"`
	Color      string
}
```

**2. Create the table** by passing the model to `Migrate` in `main.go`:

```go
if err := database.Migrate(db, &models.Project{}); err != nil {
```

**3. A route** in `main.go` (Go 1.22+ routing: method + path, `{id}` is a path parameter):

```go
mux.HandleFunc("GET /projects", func(w http.ResponseWriter, r *http.Request) { ... })
mux.HandleFunc("POST /projects", ...)
mux.HandleFunc("GET /projects/{id}", ...)   // r.PathValue("id")
```

**GORM cheat sheet:**

| Prisma-style | GORM |
|---|---|
| `create({ data })` | `db.Create(&project)` |
| `findMany()` | `db.Find(&projects)` |
| `findUnique({ where: { id } })` | `db.First(&project, id)` |
| `findMany({ where: { name } })` | `db.Where("name = ?", name).Find(&projects)` |
| `update(...)` | `db.Model(&project).Updates(models.Project{Name: "x"})` |
| `delete(...)` | `db.Delete(&project, id)` (soft delete with `gorm.Model`) |

Every call returns a `*gorm.DB`; check `.Error`, e.g. `if err := db.First(&p, id).Error; err != nil { ... }`. A missing row is `gorm.ErrRecordNotFound`.

Docs: [GORM](https://gorm.io/docs/), [net/http routing](https://pkg.go.dev/net/http#ServeMux).
