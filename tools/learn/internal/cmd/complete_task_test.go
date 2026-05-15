package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

// setupStore opens a fresh store under t.TempDir and registers cleanup.
func setupStore(t *testing.T) (string, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	st, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	t.Cleanup(func() { _ = st.Close() })
	return dbPath, st
}

// fixedNow returns a deterministic time.Now stub.
func fixedNow(ts string) func() time.Time {
	parsed, _ := time.Parse(time.RFC3339, ts)
	return func() time.Time { return parsed }
}

// TC-UC-03 (REQ-1): runCompleteTask on a fresh store inserts an events row
// with all fields populated.
func TestRunCompleteTask_insertsEventRow(t *testing.T) {
	dbPath, st := setupStore(t)
	cfg := config.DefaultConfig()
	var stderr bytes.Buffer

	const ts = "2026-05-14T12:34:56Z"
	runErr := runCompleteTask(context.Background(), completeOpts{
		DBPath:     dbPath,
		SpecPath:   ".specs/foo.md",
		SessionID:  "sess-abc",
		CommitSHA:  "deadbeef",
		TasksCount: 7,
		Config:     cfg,
		Stderr:     &stderr,
		Now:        fixedNow(ts),
	})
	if runErr != nil {
		t.Fatalf("runCompleteTask: %v", runErr)
	}

	var ev store.Event
	queryErr := st.DB().QueryRowContext(context.Background(),
		`SELECT spec_path, session_id, completed_at, commit_sha, tasks_count, outcome FROM events`).
		Scan(&ev.SpecPath, &ev.SessionID, &ev.CompletedAt, &ev.CommitSHA, &ev.TasksCount, &ev.Outcome)
	if queryErr != nil {
		t.Fatalf("query event: %v", queryErr)
	}
	if ev.SpecPath != ".specs/foo.md" {
		t.Errorf("spec_path: got %q, want %q", ev.SpecPath, ".specs/foo.md")
	}
	if ev.SessionID != "sess-abc" {
		t.Errorf("session_id: got %q, want %q", ev.SessionID, "sess-abc")
	}
	if ev.CompletedAt != ts {
		t.Errorf("completed_at: got %q, want %q", ev.CompletedAt, ts)
	}
	if ev.CommitSHA != "deadbeef" {
		t.Errorf("commit_sha: got %q, want %q", ev.CommitSHA, "deadbeef")
	}
	if ev.TasksCount != 7 {
		t.Errorf("tasks_count: got %d, want %d", ev.TasksCount, 7)
	}
	if ev.Outcome != "success" {
		t.Errorf("outcome: got %q, want %q", ev.Outcome, "success")
	}
}

// TC-UC-04 (REQ-1, REQ-11): runCompleteTask increments the nudge counter by 1.
func TestRunCompleteTask_incrementsCounter(t *testing.T) {
	dbPath, st := setupStore(t)
	cfg := config.DefaultConfig()
	cfg.NudgeThreshold = 100 // ensure no signal so stderr stays empty
	var stderr bytes.Buffer

	before, getErr := st.GetNudgeCounter(context.Background())
	if getErr != nil {
		t.Fatalf("get counter before: %v", getErr)
	}
	if runErr := runCompleteTask(context.Background(), completeOpts{
		DBPath:    dbPath,
		SpecPath:  ".specs/foo.md",
		SessionID: "sess-1",
		Config:    cfg,
		Stderr:    &stderr,
		Now:       fixedNow("2026-05-14T00:00:00Z"),
	}); runErr != nil {
		t.Fatalf("runCompleteTask: %v", runErr)
	}
	after, getErr2 := st.GetNudgeCounter(context.Background())
	if getErr2 != nil {
		t.Fatalf("get counter after: %v", getErr2)
	}
	if after != before+1 {
		t.Errorf("counter: got %d, want %d", after, before+1)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr when below threshold, got %q", stderr.String())
	}
}

// TC-UC-05 (REQ-11): when counter reaches threshold after increment, stderr
// emits the LEARN_NUDGE_DUE=true signal.
func TestRunCompleteTask_emitsNudgeSignalAtThreshold(t *testing.T) {
	dbPath, _ := setupStore(t)
	cfg := config.DefaultConfig()
	cfg.NudgeThreshold = 1 // first call reaches threshold
	var stderr bytes.Buffer

	if runErr := runCompleteTask(context.Background(), completeOpts{
		DBPath:    dbPath,
		SpecPath:  ".specs/foo.md",
		SessionID: "sess-1",
		Config:    cfg,
		Stderr:    &stderr,
		Now:       fixedNow("2026-05-14T00:00:00Z"),
	}); runErr != nil {
		t.Fatalf("runCompleteTask: %v", runErr)
	}
	if !strings.Contains(stderr.String(), "LEARN_NUDGE_DUE=true") {
		t.Errorf("expected LEARN_NUDGE_DUE=true in stderr, got %q", stderr.String())
	}
	// TASK-22: the signal must also expose the counter value so the
	// stop-learn.sh hook can surface "<N> specs since last nudge".
	if !strings.Contains(stderr.String(), "counter=1") {
		t.Errorf("expected counter=1 in stderr, got %q", stderr.String())
	}
}

// boundaryCase drives TC-UC-06a/b/c: precounter is bumped by manual UPDATEs,
// then a single runCompleteTask call increments by one and we assert whether
// the signal was emitted.
func runBoundary(t *testing.T, preCounter, threshold int, wantSignal bool) {
	t.Helper()
	dbPath, st := setupStore(t)
	if preCounter > 0 {
		if _, execErr := st.DB().ExecContext(context.Background(),
			`UPDATE nudge_state SET counter = ? WHERE id = 1`, preCounter); execErr != nil {
			t.Fatalf("seed counter: %v", execErr)
		}
	}
	cfg := config.DefaultConfig()
	cfg.NudgeThreshold = threshold
	var stderr bytes.Buffer

	if runErr := runCompleteTask(context.Background(), completeOpts{
		DBPath:    dbPath,
		SpecPath:  ".specs/foo.md",
		SessionID: "sess-1",
		Config:    cfg,
		Stderr:    &stderr,
		Now:       fixedNow("2026-05-14T00:00:00Z"),
	}); runErr != nil {
		t.Fatalf("runCompleteTask: %v", runErr)
	}
	got := strings.Contains(stderr.String(), "LEARN_NUDGE_DUE=true")
	if got != wantSignal {
		t.Errorf("signal: got %v, want %v (pre=%d threshold=%d stderr=%q)",
			got, wantSignal, preCounter, threshold, stderr.String())
	}
}

// TC-UC-06a: pre=3, threshold=5 -> counter becomes 4, no signal.
func TestRunCompleteTask_belowThreshold_noSignal(t *testing.T) {
	runBoundary(t, 3, 5, false)
}

// TC-UC-06b: pre=4, threshold=5 -> counter becomes 5, signal emitted.
func TestRunCompleteTask_atThreshold_emitsSignal(t *testing.T) {
	runBoundary(t, 4, 5, true)
}

// TC-UC-06c: pre=5, threshold=5 -> counter becomes 6, still emits (>=).
func TestRunCompleteTask_aboveThreshold_emitsSignal(t *testing.T) {
	runBoundary(t, 5, 5, true)
}

// TC-UC-06e (concurrency): two goroutines calling runCompleteTask in parallel
// on the same store finish with counter == 2 (no lost update).
func TestRunCompleteTask_concurrentIncrements_noLostUpdate(t *testing.T) {
	dbPath, st := setupStore(t)
	cfg := config.DefaultConfig()
	cfg.NudgeThreshold = 100 // suppress signal noise
	var (
		stderr bytes.Buffer
		mu     sync.Mutex
		wg     sync.WaitGroup
		errs   []error
	)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			var buf bytes.Buffer
			err := runCompleteTask(context.Background(), completeOpts{
				DBPath:    dbPath,
				SpecPath:  ".specs/foo.md",
				SessionID: "sess-" + string(rune('a'+i)),
				Config:    cfg,
				Stderr:    &buf,
				Now:       fixedNow("2026-05-14T00:00:00Z"),
			})
			mu.Lock()
			defer mu.Unlock()
			stderr.Write(buf.Bytes())
			if err != nil {
				errs = append(errs, err)
			}
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("concurrent runCompleteTask errors: %v", errs)
	}
	got, getErr := st.GetNudgeCounter(context.Background())
	if getErr != nil {
		t.Fatalf("get counter: %v", getErr)
	}
	if got != 2 {
		t.Errorf("counter after 2 concurrent calls: got %d, want 2", got)
	}
}
