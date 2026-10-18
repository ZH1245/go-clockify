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
  models/                   GORM structs = tables (Task, TaskItem) + small helpers (TotalHours, IsRunning)
  controllers/              plain functions: input → (result, error); all database logic lives here
  routes/                   HTTP: read params/body → call a controller → status code + JSON
  internal/config/          reads env vars (DATABASE_URL is required)
  internal/database/        GORM connection + Migrate()
  Dockerfile                dev target (air live reload) and prod target (static binary)
frontend/
  src/lib/api.ts            typed fetch calls to the Go API
  src/lib/time.ts           durations, including the live time of a running timer
  src/components/           tracker.tsx (page state, task list), task-row.tsx (one task, its entries, manual form)
docker-compose.yml          db + backend + frontend
```

**Request flow:** `routes` (HTTP) → `controllers` (logic + GORM) → `models` (tables). Controllers never touch HTTP;
they return errors like `ErrInvalidInput`, and `routes/response.go` turns them into 400 / 404 / 409.

## API

| Method | Path | Body | Does |
|---|---|---|---|
| GET | `/tasks` | | all tasks with their entries, `total_hours`, `timer_count`, `running` |
| GET | `/tasks/{id}` | | one task, same shape |
| POST | `/tasks` | `{"title": "…"}` | create (titles are unique → 409 on a duplicate) |
| PATCH | `/tasks/{id}` | `{"title"?: "…", "status"?: "todo" \| "in_progress" \| "done"}` | update |
| DELETE | `/tasks/{id}` | | delete the task and its entries |
| POST | `/tasks/{id}/start` | | **auto entry**: start a timer now. Several tasks can run at once; a task already running → 409 |
| POST | `/items/{id}/stop` | | stop a running timer → hours = end − start |
| POST | `/tasks/{id}/items` | `{"start_at": "2026-10-12T09:00:00Z", "manual_hours": 1.5}` | **manual entry** |
| PATCH | `/items/{id}` | `{"start_at": "…", "end_at"?: "…", "manual_hours"?: 1.5}` | edit an entry (end_at **or** manual_hours) |
| DELETE | `/items/{id}` | | delete an entry |

`total_hours` leaves out a running timer; the UI adds its live time from `start_at`.

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
