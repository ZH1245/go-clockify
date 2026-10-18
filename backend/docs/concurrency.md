# Concurrency in this project: context, WaitGroup, channels

This is the "why" behind the concurrent code in the repo. Read it next to the files it
points at; every snippet is either real code here or a variant of it.

| Tool | Where it lives here | What it is for |
|---|---|---|
| `context.Context` | every controller, `routes/dashboard.route.go`, `cmd/server/main.go` | cancellation, deadlines, and request-scoped values |
| `signal.Notify` channel | `cmd/server/main.go` (shutdown) | one goroutine telling another "stop" |
| `sync.WaitGroup` | `controllers/dashboard.controller.go` | wait for several goroutines to finish |
| `time.Ticker` + `ctx.Done()` | `internal/sweeper/sweeper.go` | periodic background work |
| channels (fan-in / signal) | `controllers/dashboard.controller.go`, `cmd/server/main.go`, `internal/sweeper/sweeper.go` | collect results / signal "stop" |

Everything here is the standard library. The only third-party package in the backend is
GORM (the database layer).

---

## 1. `context.Context` — cancellation and deadlines

A context carries one thing: a *done* signal. Anything doing work that can be cancelled
(a DB query, an HTTP call, a long computation) should accept one and watch it.

**Where it comes from.** Every `http.Handler` gets an `*http.Request`, and
`r.Context()` is cancelled the moment the client disconnects or the server shuts down.
That is the root of the tree for a request:

```
r.Context()                     ← cancelled when the client goes away
  └─ context.WithTimeout(...5s) ← also cancelled after 5s   (routes/dashboard.route.go)
       ├─ SELECT ...running timers...   (goroutine)
       └─ SELECT ...totals...           (goroutine)
```

**Where we use it.** Every controller takes `ctx` first and hands it to GORM:

```go
func GetTasks(ctx context.Context, db *gorm.DB) ([]models.Task, error) {
    err := db.WithContext(ctx).Preload("Project").Find(&tasks).Error
    //     ^^^^^^^^^^^^^^^ ties the query to the request's lifetime
}
```

`db.WithContext(ctx)` is the whole point: if the browser tab closes mid-query, Postgres
abandons the work instead of finishing it for nobody, and a stuck query cannot hold a
connection forever.

**The rules that matter (from the Go docs, condensed):**

- Pass `ctx` as the **first parameter**, named `ctx`. Do not put a `Context` in a struct field.
- Never pass `nil`; if you do not have one, use `context.TODO()` while you work it out.
- When you derive one, **always `defer cancel()`** — otherwise you leak the timer/goroutine:

  ```go
  ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
  defer cancel()
  ```

- Treat the context value API (`context.WithValue`) as a last resort for request-scoped
  data (auth user, trace id). Never for optional function arguments.

**Reading the shutdown path in `cmd/server/main.go`** is the best 20 lines in the repo
for this: the server runs in a goroutine, `main` blocks on a signal channel, then
`server.Shutdown(ctx)` is given a 10-second context so in-flight requests can finish
before the process exits.

---

## 2. `sync.WaitGroup` — "wait until these goroutines are done"

A WaitGroup is a counter. The contract is three methods:

```go
var wg sync.WaitGroup

wg.Add(1)            // BEFORE starting the goroutine: "one more to wait for"
go func() {
    defer wg.Done()  // the goroutine says "I finished"
    doWork()
}()

wg.Wait()            // blocks until the counter hits zero
```

**The classic bugs:**

- `wg.Add(1)` *inside* the goroutine: `Wait` can return before `Add` runs. Always call
  `Add` before `go`.
- Forgetting `Done` on an early `return` → `Wait` hangs forever. `defer wg.Done()`.
- Reusing a `WaitGroup` before the previous `Wait` returned.

**Where a plain WaitGroup fits here.** The dashboard is exactly this shape: two independent
queries that both need to finish before we can answer. Section 3 shows the real code; the
same pattern without error handling is simply:

```go
func warmUp(ctx context.Context, db *gorm.DB, ids []uint) {
    var wg sync.WaitGroup
    for _, id := range ids {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _ = db.WithContext(ctx).First(&models.Project{}, id).Error
        }()
    }
    wg.Wait()
}
```

Note `id` is captured directly: since Go 1.22 each loop iteration has its **own** `id`,
so you do not need the old `id := id` trick. (This repo is on `go 1.27`.)

---

## 3. The dashboard: a WaitGroup plus a channel

`controllers/dashboard.controller.go` is the real, working example. Two independent queries,
so two goroutines, and a channel to carry their errors back:

```go
var wg sync.WaitGroup
errs := make(chan error, 2)   // one slot per goroutine

wg.Add(2)
go func() {
    defer wg.Done()
    errs <- db.WithContext(ctx).Raw(runningTimersSQL).Scan(&running).Error
}()
go func() {
    defer wg.Done()
    errs <- db.WithContext(ctx).Raw(dashboardTotalsSQL, startOfDay, startOfWeek).Scan(&totals).Error
}()

wg.Wait()
close(errs)

for err := range errs {
    if err != nil {
        return Dashboard{}, err
    }
}
```

Why each piece is the way it is:

- `errs` is **buffered with exactly 2 slots**, so neither goroutine ever blocks on send.
  Make it unbuffered and both goroutines sit waiting for a receiver while `wg.Wait()` waits
  for them — a deadlock. This is the most common bug in this pattern.
- We `close(errs)` only **after** `wg.Wait()`, so the `range` below is guaranteed to see
  every send. Rule: the side that knows the senders are finished closes the channel; a
  receiver never closes it.
- `defer wg.Done()` rather than a bare `wg.Done()` at the end, so the counter drops even if
  the query panics.
- The two goroutines write to two **different** variables (`running`, `totals`), which is
  safe. Two goroutines writing the same variable is a data race — `go test -race` catches it.
- Nothing here cancels the *other* query when one fails; `wg.Wait()` still waits for both.
  That is the one thing `errgroup` adds for free (see the note below).

> **If you did not mind an extra dependency**, `golang.org/x/sync/errgroup` packages up all
> of the above: `g.Go(fn)` instead of `Add`/`Done`, `g.Wait()` returns the first error, and
> it hands back a context that is cancelled as soon as any goroutine fails. It is maintained
> by the Go team and widely used. This repo deliberately does **not** depend on it — the
> standard-library version is only a few lines longer and makes the mechanics visible.

> **Honest note:** two fast queries on the same database often finish no faster when run
> in parallel, and sequential code is easier to read. The dashboard does it concurrently
> because the queries are independent and it is a clean place to practise the pattern.
> Reach for concurrency when work is genuinely independent and slow, not by default.

---

## 4. Channels — the other way goroutines talk

"Don't communicate by sharing memory; share memory by communicating." A channel is a
typed pipe between goroutines.

**We already use one, in the shutdown path:**

```go
stop := make(chan os.Signal, 1)          // buffered: the signal package sends without blocking
signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
<-stop                                    // receive blocks until a signal arrives
```

`signal.Notify` needs a **buffered** channel: the runtime sends the signal whether or not
anyone is receiving yet, and an unbuffered channel would drop it.

**Fan-in: many goroutines, one collector.** The dashboard (section 3) does the minimal
version: it sends only *errors* down the channel and writes the results straight into two
variables. When you want to carry whole results instead, send a struct down the channel and
range over it. The `WaitGroup`'s only job is to close the channel once every producer is done:

```go
type result struct {
    running []RunningTimer
    totals  DashboardTotals
    err     error
}

func getDashboardWithChannels(ctx context.Context, db *gorm.DB) (Dashboard, error) {
    results := make(chan result, 2)   // buffered: one slot per producer
    var wg sync.WaitGroup

    wg.Add(2)
    go func() {
        defer wg.Done()
        var running []RunningTimer
        err := db.WithContext(ctx).Raw(runningTimersSQL).Scan(&running).Error
        results <- result{running: running, err: err}
    }()
    go func() {
        defer wg.Done()
        var totals DashboardTotals
        err := db.WithContext(ctx).Raw(dashboardTotalsSQL).Scan(&totals).Error
        results <- result{totals: totals, err: err}
    }()

    wg.Wait()       // every producer has finished sending
    close(results)  // safe now, and closing is what ends the range below

    var (
        dashboard Dashboard
        firstErr  error
    )
    for r := range results {          // runs twice, then the channel is closed and the loop exits
        if r.err != nil && firstErr == nil {
            firstErr = r.err
        }
        if r.running != nil {
            dashboard.RunningTimers = r.running
        }
        dashboard.Totals = r.totals
    }
    return dashboard, firstErr
}
```

Two rules that make this safe:

- **Buffer with the number of producers.** `make(chan result, 2)` for two goroutines means
  a send never blocks, which is what keeps this pattern deadlock-free. An unbuffered channel
  only works if a receiver is already ranging — see the deadlock note in section 3.
- **Close once all senders are done** (here, right after `wg.Wait()`), so the `range`
  terminates. Nothing may send after a `close`. A receiver never closes the channel.

The dashboard in section 3 is the small version of this idea: it relies on `wg.Wait()` and a
buffered error channel, and writes results into plain variables. Sending whole results down a
channel earns its keep when the shape is different — pipelines, worker pools, streaming
results, or a long-lived event loop.

**`select`** is how you wait on several channels at once:

```go
select {
case r := <-results:
    handle(r)
case <-ctx.Done():                 // e.g. the request was cancelled
    return ctx.Err()
}
```

### One more shape: a goroutine that never returns (the sweeper)

`internal/sweeper/sweeper.go` does not "run N things and wait". It is "run one thing,
forever, until told to stop": it closes timers whose browser stopped pinging.

```go
go func() {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    sweep(ctx, db, timeout)           // once at startup, then on every tick

    for {
        select {
        case <-ctx.Done():
            return                    // shutdown: end the goroutine
        case <-ticker.C:
            sweep(ctx, db, timeout)
        }
    }
}()
```

How it differs from the patterns above:

- **Nobody is waiting on it.** No result channel, no `WaitGroup`, because no request is
  blocked on the outcome. A channel here would be bookkeeping to nowhere. The only
  coordination is `ctx.Done()`.
- **Its context is not a request context.** `main` does
  `context.WithCancel(context.Background())` and calls `cancel()` on shutdown — the goroutine
  has to outlive every request, so it cannot borrow one's context.
- **A `time.Ticker` is just a channel** (`ticker.C`) that fires on a schedule. `defer
  ticker.Stop()` is required; an unstopped ticker leaks. `time.NewTicker` *panics* if the
  interval is `<= 0`, which is why `Start` checks that first (and exits cleanly, so
  `SWEEP_INTERVAL=0` disables the sweeper).
- **Each run gets its own timeout** (`context.WithTimeout(ctx, 5*time.Second)`) so one slow
  query cannot stall the next tick.

---

## 5. Rules of thumb

- **context**: first arg of anything that does I/O; cancel what you derive; never store it.
- **WaitGroup**: fan-out over several goroutines. `Add` before `go`, `defer Done`, one `Wait`.
- **channel + WaitGroup together**: when each goroutine can fail, buffer the channel with one
  slot per goroutine, `Wait`, `close`, then drain. This is what the dashboard does.
- **goroutine + ticker + ctx.Done()**: background work on a schedule. Nobody waits for it, so
  no WaitGroup and no result channel — just a cancellable context and a stopped ticker.
- **channels**: pipelines, worker pools, streaming, signalling. Close once all senders are
  done. Prefer them over sharing a slice with a mutex when the data flows one way.
- **Don't guess**: `go test -race ./...` catches data races, and
  `go run -race ./cmd/server` catches them while you click around.

## 6. Things to try (small, real exercises)

1. Make `GetDashboard`'s second query fail on purpose (e.g. select from a missing table) and
   confirm the endpoint still returns an error and does not hang. Then change `errs` to
   `make(chan error)` (unbuffered) and watch the request hang — that makes the buffering
   requirement concrete instead of theoretical.
2. Add a `/dashboard/summary` endpoint that returns only the totals, written sequentially,
   and time both versions. If parallel is not measurably faster, that is useful to know.
3. Wrap the running-timers query with a 1-second `context.WithTimeout` and a `pg_sleep(2)`
   in the SQL to watch the query be abandoned and the handler fail.
4. Run the backend with `-race` and add a deliberate shared-slice bug (e.g. both goroutines
   `append` to the same slice) to see the race detector fire.
