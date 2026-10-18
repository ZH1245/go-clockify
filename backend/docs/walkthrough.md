# Backend walkthrough

A guided tour of the Go API, written so you can explain it line by line without re-reading
everything first. Read it next to the code. Pair it with
[`concurrency.md`](concurrency.md), which goes deeper on `context`, `WaitGroup` and channels.

```
request  →  routes/  →  controllers/  →  models/  →  Postgres
             (HTTP)      (decisions)      (shape)
```

Each layer has one job:

| Layer | Job | Knows about |
|---|---|---|
| `routes/` | parse HTTP, map errors to status codes, write JSON | `net/http`, status codes |
| `controllers/` | decide what should happen, run the queries | GORM, business rules |
| `models/` | describe the tables and the derived numbers | GORM tags, no SQL |
| `internal/` | process wiring: config, connection, background work | startup concerns |

The rule that keeps it clean: **controllers never import `net/http`.** They return Go values
and errors; `routes/` turns those into status codes. That is why `writeError` lives in
`routes/response.go` and not in a controller.

---

## 1. Follow one request end to end

`POST /tasks/5/start` — start the clock on task 5.

**`routes/task_item.route.go`**

```go
mux.HandleFunc("POST /tasks/{id}/start", func(w http.ResponseWriter, r *http.Request) {
    taskID, err := pathID(r)                      // "5" → uint(5)
    if err != nil {
        writeError(w, err)                        // 400
        return
    }
    item, err := controllers.StartTimer(r.Context(), db, taskID)
    if err != nil {
        writeError(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, item)
})
```

Three things to notice:

- The pattern `"POST /tasks/{id}/start"` is Go 1.22+ `ServeMux` syntax: the method is part
  of the pattern and `{id}` is a wildcard. No router library needed.
- `r.Context()` is threaded straight in. That context is cancelled the moment the client
  disconnects or the server shuts down, and it carries the request's deadline down to the
  database driver. Nothing else in the handler has to care.
- `pathID` and `writeError` are the only HTTP-shaped helpers. Everything else is Go.

**`controllers/task_item.controller.go`** — `StartTimer`

```go
err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    var task models.Task
    if err := tx.First(&task, taskID).Error; err != nil {
        return err                                   // no such task → 404
    }

    var running int64
    if err := tx.Model(&models.TaskItem{}).
        Where("task_id = ? AND "+runningItems, taskID).Count(&running).Error; err != nil {
        return err
    }
    if running > 0 {
        return fmt.Errorf("%w: this task already has a running timer", ErrConflict)  // 409
    }

    now := time.Now().UTC()
    item = models.TaskItem{TaskID: taskID, StartAt: now, LastPingAt: &now}
    if err := tx.Create(&item).Error; err != nil {
        return err
    }
    return tx.Model(&task).Update("status", models.TaskStatusInProgress).Error
})
```

- **Why a transaction:** "create the time entry" and "flip the task to in_progress" must
  both happen or neither. `Transaction` commits if the closure returns `nil` and rolls back
  on any error, so there is no way to leave a task marked in-progress with no timer.
- **`runningItems`** is the shared predicate that defines a running timer:
  `"end_at IS NULL AND manual_hours IS NULL"`. Used here, in `PingTimers`, in
  `SweepStaleTimers` and in the dashboard SQL.
- **`fmt.Errorf("%w: ...", ErrConflict)`** wraps a sentinel error. `writeError` later asks
  `errors.Is(err, controllers.ErrConflict)` and answers 409. This is how a controller
  reports an HTTP outcome without importing HTTP.

**`routes/response.go`** — the error → status mapping, in one place:

| Controller error | Status |
|---|---|
| `gorm.ErrRecordNotFound` | 404 |
| `ErrInvalidInput` | 400 |
| `ErrConflict` | 409 |
| anything else | 500 (and the real error is logged, never sent to the client) |

---

## 2. File by file

### `cmd/server/main.go` — the composition root

Startup order matters and is easy to defend:

1. `config.Load()` — read env, fail fast on a missing `DATABASE_URL`.
2. `database.Connect(...)` — one `*gorm.DB` for the whole process, safe for concurrent use
   because it wraps a `database/sql` connection pool.
3. `database.Migrate(db, &models.Project{}, &models.Task{}, &models.TaskItem{})` —
   **Project first**, because `Task` has a foreign key to it.
4. Build the mux, register routes, wrap with CORS.
5. `sweeper.Start(...)` — background work, started after the DB is ready.
6. `server.ListenAndServe()` in a goroutine, so `main` can block on a signal.
7. `<-stop` — Ctrl+C (SIGINT) or `docker compose stop` (SIGTERM).
8. `stopSweeper()`, then `server.Shutdown(ctx)` with a 10s budget, then close the pool.

```go
go func() {
    log.Printf("listening on :%s", cfg.Port)
    if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Fatalf("server: %v", err)
    }
}()

stop := make(chan os.Signal, 1)
signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
<-stop
```

- `ListenAndServe` **always** returns an error when it stops, including on a clean shutdown
  (`http.ErrServerClosed`). That is why the check is `err != nil && !errors.Is(..., ErrServerClosed)`
  — otherwise every graceful shutdown would exit with a fatal log.
- The signal channel is buffered (`make(chan os.Signal, 1)`) so the runtime can deliver a
  signal even if `main` has not reached the receive yet.
- `Shutdown` stops accepting new connections and waits for in-flight requests; it does not
  kill them. That is the difference between `Shutdown` (graceful) and `Close` (immediate).

### `cmd/server/cors.go`

`/tasks` and `/timers/ping` are called from a browser on a different origin (`:3001` →
`:8090`), so the responses need `Access-Control-Allow-Origin`. `OPTIONS` preflights are
answered with 204 and never reach the mux. The allowed origin comes from config rather than
`*`, which is the difference between "works locally" and "safe to deploy".

### `internal/config/config.go`

```go
TimerPingTimeout time.Duration  // TIMER_PING_TIMEOUT, default 2m
SweepInterval    time.Duration  // SWEEP_INTERVAL,      default 30s
```

`getDuration` parses `"90s"` / `"2m"` and **falls back with a log line on a typo** instead of
crashing. A bad optional env var should not take the API down; a missing `DATABASE_URL`
should, which is why that one is `mustEnv` and calls `log.Fatalf`.

### `internal/database/database.go`

```go
gorm.Open(postgres.Open(databaseURL), &gorm.Config{TranslateError: true})
```

`TranslateError: true` is load-bearing. Without it a unique-index violation arrives as a raw
driver error and controllers cannot tell it apart from a broken connection. With it, GORM
returns `gorm.ErrDuplicatedKey`, so `duplicateTitleError` can map a duplicate task or project
title to 409 instead of 500.

`Migrate` is a one-line wrapper over `AutoMigrate`. Note what `AutoMigrate` does and does not
do: it **adds** tables and columns (that is how `last_ping_at` appeared), but it does not
alter existing constraints. A `gorm:"constraint:OnDelete:SET NULL"` tag on a field that
already exists in the database is quietly ignored.

### `models/project.model.go`

```go
Title      string `gorm:"not null;uniqueIndex" json:"title"`
ClientName string `gorm:"index" json:"client_name"`
Tasks      []Task `json:"tasks,omitempty"`
```

`Title` is unique; `ClientName` is deliberately **not**. Many projects have no client, and a
unique index would reject the second `''`. `omitempty` keeps `tasks` out of list responses
rather than emitting a wall of empty arrays.

### `models/task.model.go`

```go
ProjectID *uint      `gorm:"index" json:"project_id"`
Project   *Project   `json:"project,omitempty"`
Items     []TaskItem `gorm:"constraint:OnDelete:CASCADE" json:"items"`
```

- **`*uint`, not `uint`.** A pointer is how you say "this may be NULL". A plain `uint` would
  store `0` and there is no row with id 0 — and, worse, `0` cannot be told apart from "not
  set" during an update. The same reasoning applies to `EndAt *time.Time` and
  `ManualHours *float64`: an entry is timed, manual, or running, and NULL is what
  distinguishes them.
- `Items` is `ON DELETE CASCADE`: deleting a task deletes its time entries. That is a
  deliberate (if debatable) choice — see §5.
- `Project` has no cascade rule, so the foreign key stays `NO ACTION` and Postgres refuses to
  delete a project that still has tasks. Deleting therefore *must* go through
  `DeleteProject`, which detaches first.

Derived values are **methods, not columns**:

```go
func (item TaskItem) IsRunning() bool {
    return item.EndAt == nil && item.ManualHours == nil
}
```

A stored `is_running` column could disagree with `end_at`. Computing it means the two can
never drift. `TotalHours()` returns `0` for a running item, because the UI shows live time
from `StartAt` and the server should not guess "now".

### `controllers/types.go`

All the request bodies live in one small file, so the API's input shape is readable at a
glance. `UpdateTaskInput` uses pointers (`*string`, `*TaskStatus`) so that "field absent"
(`nil`, leave unchanged) is distinguishable from "field set to a zero value".

### `controllers/task.controller.go`

Read the four functions in order and the pattern is the same every time:

1. Validate input, return `ErrInvalidInput` with a human-readable reason.
2. Do the work with `db.WithContext(ctx)`.
3. **Return the freshly-read row**, not the object you just assembled.

That third point is why `CreateTask` and `UpdateTask` end with `GetTaskByID(ctx, db, ...)`.
It costs one extra query and buys a response that always matches what is actually stored —
including the `Preload`ed project, the DB-assigned `id`, and the `created_at` the database
generated rather than the process clock.

`newestItemsFirst` deserves a mention:

```go
func newestItemsFirst(db *gorm.DB) *gorm.DB {
    return db.Order("start_at DESC")
}
```

`Preload("Items")` takes a function that can modify the query, and this one adds an
`ORDER BY`. It cannot be written as a string argument because **a placeholder cannot stand in
for an identifier or a clause** — `?` is only for values. The order has to be a literal
inside the code, which is safe precisely because it is a constant, not user input.

### `controllers/task_item.controller.go`

`StartTimer` (above), then:

- **`StopTimer`** checks `IsRunning()` first and returns 409 if not, so stopping an already
  stopped timer is a clear client error rather than a silent no-op.
- **`AddManualEntry`** validates `start_at` is present and `manual_hours` is within
  `(0, 24]`. The 24 cap is a sanity bound, not a rule about how long work can take.
- **`UpdateTaskItem`** is the subtle one:

```go
changes := models.TaskItem{StartAt: ..., EndAt: input.EndAt, ManualHours: input.ManualHours}
if err := db.WithContext(ctx).Model(&item).
    Select("start_at", "end_at", "manual_hours").Updates(changes).Error; err != nil {
```

`Updates` with a struct **skips zero values**, so a `nil` `EndAt` would be ignored and the
column left as-is — you could never clear an end time. `Select` names the columns to write
explicitly, which forces the `NULL`s through.

- **`DeleteTaskItem`** and the others check `RowsAffected == 0` after a delete and turn it
  into `gorm.ErrRecordNotFound`. GORM treats "delete 0 rows" as success, which would give a
  misleading 204 for an id that never existed.

### `controllers/project.controller.go`

The interesting parts are the shared SQL fragment and the delete.

```go
const itemHoursSQL = `(CASE
	WHEN ti.manual_hours IS NOT NULL THEN ti.manual_hours
	WHEN ti.end_at IS NOT NULL THEN EXTRACT(EPOCH FROM (ti.end_at - ti.start_at)) / 3600.0
	ELSE 0
END)`
```

This is **the** definition of "how many hours is this row". It is used by the project list,
the single-project read, and the dashboard, in SQL. `models.TaskItem.TotalHours()` is the
same rule in Go. Two implementations of one rule is a smell — but they exist for good
reasons: the SQL version runs in the database over thousands of rows and can `SUM`/`GROUP BY`;
the Go version is used in-process for a task whose items are already loaded. They must be
kept in step, and that is worth a comment (and, if it ever drifts, a test).

`projectStatsQuery` uses `LEFT JOIN` twice — projects → tasks → items — with
`COUNT(DISTINCT tasks.id)` and `GROUP BY projects.id`. `LEFT` matters: an `INNER JOIN` would
hide every project that has no tasks yet. `DISTINCT` matters: joining to items multiplies the
task rows, so a project with 3 tasks and 5 entries would otherwise report 15.

```go
COALESCE(SUM(` + itemHoursSQL + `), 0) AS total_hours
```

`SUM` over zero rows is `NULL`, and `NULL` would serialize as `null` in JSON where the
frontend expects a number.

`GetProjects` starts from `[]ProjectWithStats{}`, not `var projects []ProjectWithStats`.
A nil slice marshals to `null`; and empty slice marshals to `[]`. The frontend would have to
special-case `null` everywhere, so the API always sends an array.

And `DeleteProject`:

```go
return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    if err := tx.Model(&models.Task{}).
        Where("project_id = ?", id).
        Updates(map[string]any{"project_id": nil}).Error; err != nil {
        return err
    }
    result := tx.Delete(&models.Project{}, id)
    ...
    if result.RowsAffected == 0 {
        return gorm.ErrRecordNotFound
    }
    return nil
})
```

Detach, then delete, atomically. `Updates(map[string]any{"project_id": nil})` is a map rather
than a struct specifically so the `nil` is written as SQL `NULL` instead of being skipped as
a zero value.

### `controllers/dashboard.controller.go`

Two independent queries, run at the same time, with the standard library only:

```go
var wg sync.WaitGroup
errs := make(chan error, 2)   // one slot per goroutine

wg.Add(2)
go func() { defer wg.Done(); errs <- db.WithContext(ctx).Raw(runningTimersSQL).Scan(&running).Error }()
go func() { defer wg.Done(); errs <- db.WithContext(ctx).Raw(dashboardTotalsSQL, startOfDay, startOfWeek).Scan(&totals).Error }()

wg.Wait()
close(errs)
for err := range errs {
    if err != nil {
        return Dashboard{}, err
    }
}
```

The buffered channel is the part to get right: with `make(chan error)` (unbuffered) both
goroutines would block on send while `wg.Wait()` waits for them — a deadlock. Two goroutines
writing to two different variables (`running`, `totals`) is safe; two goroutines writing the
*same* variable would be a data race.

The SQL is `Raw`, not the ORM builder, because these are aggregates with `FILTER (WHERE ...)`
clauses — Postgres-specific work that has nothing to do with a table struct. The values
(`startOfDay`, `startOfWeek`) are still passed as placeholders, so they travel separately
from the SQL text.

The week boundary is computed in Go:

```go
weekdayOffset := (int(startOfDay.Weekday()) + 6) % 7   // Monday is 0
startOfWeek := startOfDay.AddDate(0, 0, -weekdayOffset)
```

Go's `time.Weekday` counts from Sunday (0). The `+6) % 7` shifts it so Monday is 0, which is
what the UI means by "this week".

### `controllers/timer.controller.go` + `internal/sweeper/sweeper.go`

See §4.

### `routes/response.go`

`writeJSON`, `writeError`, `pathID`, `readJSON`. Small and total: every handler uses
`pathID` for path params and `readJSON` for bodies, so malformed input is a 400 with a
message in one consistent shape, `{"error": "..."}`.

---

## 3. Concurrency, in one place

Three shapes, all standard library. The full explanation is in
[`concurrency.md`](concurrency.md); this is the index:

| Shape | Where | Why |
|---|---|---|
| `context.Context` | every controller, `routes/dashboard.route.go`, `main.go` | cancellation, deadlines, request scope |
| `sync.WaitGroup` + buffered channel | `controllers/dashboard.controller.go` | two independent queries, both can fail |
| long-lived goroutine + `time.Ticker` + `ctx.Done()` | `internal/sweeper/sweeper.go` | periodic work with no request waiting on it |
| `signal.Notify` channel | `cmd/server/main.go` | one goroutine telling another to stop |

`sync.WaitGroup` is the only synchronisation primitive used. There is no mutex anywhere,
because there is no shared mutable state: each request gets its own values, and the
database is the shared state, with its own locking.

---

## 4. The ping / heartbeat system

**The problem.** A timer starts recording when the browser asks. If the user closes the tab
(or the laptop dies), nothing ever calls `POST /items/{id}/stop`, and the timer runs forever,
inflating every total.

**The design.** The timer's deadline is derived from data, not held in memory:

```
stop at  =  last_ping_at + TIMER_PING_TIMEOUT
```

Every ping moves `last_ping_at` forward, so "the interval restarts on each ping" falls out of
a single column.

- `POST /timers/ping` (`controllers.PingTimers`) runs one `UPDATE`:

  ```go
  db.WithContext(ctx).Model(&models.TaskItem{}).
      Where(runningItems).
      Update("last_ping_at", time.Now().UTC())
  ```

  It refreshes **every** running timer, because the app has no user or session concept. Once
  auth exists this becomes per-user.

- `StartTimer` seeds `LastPingAt` at creation, so a timer with no ping at all cannot exist.

- The sweeper (`internal/sweeper`) ticks every `SWEEP_INTERVAL`:

  ```go
  db.WithContext(ctx).Model(&models.TaskItem{}).
      Where(runningItems+" AND last_ping_at IS NOT NULL AND last_ping_at < ?", cutoff).
      Update("end_at", gorm.Expr("last_ping_at"))
  ```

Three details worth defending:

1. **Check and write are one statement.** There is no read-then-write window, so no lock is
   needed. Either a ping lands first and the row is skipped, or it lands second and finds
   `end_at` already set while its own `WHERE` matches nothing. Both orders are correct.
2. **`end_at = last_ping_at`, not `now()`.** The user stopped working when the browser went
   away; the gap between the last ping and the sweep is not their time. This bounds the
   error to one ping interval instead of the whole sweep interval.
3. **`last_ping_at IS NOT NULL` is never swept.** "Never pinged" is not "pinged too long
   ago" — rows predating this feature keep running rather than being silently closed.

Measured behaviour (timeout 2m, sweep 30s):

| Event | Time |
|---|---|
| start | `15:58:42` |
| last ping | `15:58:50` |
| deadline | `16:00:50` |
| row closed by sweeper | `16:01:04` |
| `end_at` written | `15:58:50` |
| time booked | `00:00:08.06` |

So the row closes up to one sweep interval late, but **not one second past the last ping is
ever booked.**

**Why not `time.Timer` with `Reset()` per timer?** That fires exactly on the deadline, but the
countdown lives in process memory. Restart the server and every pending deadline is gone, so
a timer that should have stopped runs forever — and you would need a database sweep at
startup to recover, which is this mechanism anyway. The derived version cannot disagree with
the database. (It also breaks the moment you run two instances of the API.)

**Tuning.** Ping every 15s from the browser; timeout defaults to 2m. The margin exists
because browsers throttle timers in hidden tabs down to roughly once a minute — a short
timeout would stop timers belonging to tabs that are open but in the background.
`SWEEP_INTERVAL=0` disables the sweeper entirely, which is useful for testing.

---

## 5. Decisions, and the reasoning to give

**Why does deleting a project not delete its time?**
Time is the product; a project is an organisational label. `DeleteProject` detaches tasks
(`project_id = NULL`) and deletes only the project row, in one transaction. The foreign key
is left as `NO ACTION` deliberately so that no single `DELETE` can destroy hours; the
destructive path is explicit and reviewable.

**Why is `PUT /tasks/{id}/project` a separate endpoint instead of part of `PATCH /tasks/{id}`?**
On a `*uint`, "field absent" and "field is `null`" are the same thing: `nil`. `PATCH` needs
absent to mean "leave alone", so it cannot also carry "clear it". A dedicated endpoint makes
the intent explicit and the body unambiguous.

**Why is `ProjectWithStats` different from `models.Project`?**
`models.Project` is the table. `task_count` and `total_hours` are aggregates that no row
holds, so they are composed on top in the controller and only where they are cheap. `POST`
and `PATCH /projects` return the bare row; `GET /projects` returns the row plus stats. That
is a deliberate difference (writing does not need a stats query), and it is the kind of thing
a reviewer will ask about, so it is written down here.

**Why check `RowsAffected == 0` on deletes?**
GORM reports success for a delete that matched nothing. Without the check, `DELETE /tasks/999999`
would answer 204 and the client would believe something was deleted.

**Why is `Task.Project` eagerly loaded with `Preload`?**
A task without its project would serialize as `"project": null` even when `project_id` is
set — because GORM only fills the field if you ask. `Preload("Project")` runs one extra query
for the whole list, rather than one per task (no N+1).

**Why return 409 rather than 400 for a duplicate title or a double-started timer?**
The request is well-formed; it conflicts with current state. 400 means "your request is
wrong", 409 means "the server's state says no".

---

## 6. API reference

| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/health` | — | pings the pool, 503 if the DB is down |
| GET | `/tasks` | — | includes `project`, `items`, `total_hours`, `running` |
| POST | `/tasks` | `{"title","project_id"}` | 409 on duplicate title |
| GET | `/tasks/{id}` | — | |
| PATCH | `/tasks/{id}` | `{"title","status"}` | omitted fields untouched |
| DELETE | `/tasks/{id}` | — | also deletes its items (CASCADE) |
| PUT | `/tasks/{id}/project` | `{"project_id": 3}` or `null` | set or clear the project |
| POST | `/tasks/{id}/start` | — | 409 if already running |
| POST | `/tasks/{id}/items` | `{"start_at","manual_hours"}` | manual entry |
| POST | `/items/{id}/stop` | — | sets `end_at = now()` |
| PATCH | `/items/{id}` | `{"start_at","end_at","manual_hours"}` | |
| DELETE | `/items/{id}` | — | |
| GET | `/projects` | — | each row + `task_count`, `total_hours` |
| GET | `/projects/{id}` | — | |
| POST | `/projects` | `{"title","client_name"}` | 409 on duplicate title |
| PATCH | `/projects/{id}` | `{"title","client_name"}` | |
| DELETE | `/projects/{id}` | — | keeps tasks and time, detaches them |
| GET | `/dashboard` | — | running timers + today/week/all-time hours |
| POST | `/timers/ping` | — | heartbeat; refreshes every running timer |

### Environment

| Var | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | listen port |
| `DATABASE_URL` | — (required) | Postgres DSN |
| `FRONTEND_ORIGIN` | `http://localhost:3001` | the one origin allowed by CORS |
| `TIMER_PING_TIMEOUT` | `2m` | a timer with no ping for this long is stopped |
| `SWEEP_INTERVAL` | `30s` | how often the sweeper looks (`0` disables it) |

---

## 7. Known limitations, stated honestly

A reviewer will find these; better that you raise them first.

1. **Double-start is a small race.** `StartTimer` counts running items and then inserts,
   inside one transaction. Under Postgres's default `READ COMMITTED` two simultaneous
   requests for the same task can both count `0` and both insert. The airtight fix is a
   partial unique index:
   `CREATE UNIQUE INDEX ON task_items (task_id) WHERE end_at IS NULL AND manual_hours IS NULL`.
   `AutoMigrate` cannot express that, so it needs a raw migration.
2. **"Hours" is defined twice** — `itemHoursSQL` in SQL and `TotalHours()` in Go. They must
   stay in step. A test that compares them on the same fixture would pin it down.
3. **Deleting a task destroys its tracked time** (`ON DELETE CASCADE`). Arguably a task
   deletion should soft-delete instead. Not implemented.
4. **The ping is global, not per-user**, because there is no auth. Any browser pinging keeps
   every running timer alive.
5. **The dashboard runs two queries concurrently but they hit the same database.**
   Concurrency here is a teaching device and a fair pattern for independent work; it is not
   a measured performance win.

## 8. Things worth trying next

- Add the partial unique index from limitation 1 and write a test that fires two `StartTimer`
  calls concurrently to prove it holds.
- Add `gorm.DeletedAt` to `TaskItem`/`Task` so deletes stop destroying time.
- Make `/dashboard`'s running list push-based (SSE or WebSocket) instead of polling once on
  load, so the UI does not need its own 1s tick for elapsed time.
