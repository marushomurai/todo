package store_test

import (
	"database/sql"
	"testing"

	"github.com/yuyanky/todo/internal/db"
	"github.com/yuyanky/todo/internal/store"
)

func setupStores(t *testing.T) (*store.TaskStore, *store.PlanStore, *sql.DB) {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return store.NewTaskStore(d), store.NewPlanStore(d), d
}

func TestCreatePlan(t *testing.T) {
	ts, ps, _ := setupStores(t)
	ts.Add("A")
	ts.Add("B")
	ts.Add("C")

	err := ps.CreatePlan("2026-03-12", []int64{1, 3})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := ps.GetPlan("2026-03-12")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("plan should exist")
	}
	if plan.State != "closed" {
		t.Errorf("got state %q, want closed", plan.State)
	}

	items, err := ps.TodayItems("2026-03-12")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Task.Title != "A" {
		t.Errorf("got %q, want A", items[0].Task.Title)
	}
	if items[1].Task.Title != "C" {
		t.Errorf("got %q, want C", items[1].Task.Title)
	}
}

func TestCreatePlan_DuplicateDate(t *testing.T) {
	ts, ps, _ := setupStores(t)
	ts.Add("A")
	ps.CreatePlan("2026-03-12", []int64{1})

	err := ps.CreatePlan("2026-03-12", []int64{1})
	if err == nil {
		t.Error("expected error for duplicate plan date")
	}
}

func TestMarkDone(t *testing.T) {
	ts, ps, _ := setupStores(t)
	ts.Add("A")
	ps.CreatePlan("2026-03-12", []int64{1})

	err := ps.MarkDone("2026-03-12", 1)
	if err != nil {
		t.Fatal(err)
	}

	items, _ := ps.TodayItems("2026-03-12")
	if items[0].Disposition != "done" {
		t.Errorf("got disposition %q, want done", items[0].Disposition)
	}
}

func TestMarkDone_NotInPlan(t *testing.T) {
	_, ps, _ := setupStores(t)
	err := ps.MarkDone("2026-03-12", 999)
	if err == nil {
		t.Error("expected error for task not in plan")
	}
}

func TestReview(t *testing.T) {
	ts, ps, _ := setupStores(t)
	ts.Add("A")
	ts.Add("B")
	ts.Add("C")
	ps.CreatePlan("2026-03-12", []int64{1, 2, 3})

	// Complete one task
	ts.Done(1)
	ps.MarkDone("2026-03-12", 1)

	done, carriedOver, err := ps.Review("2026-03-12")
	if err != nil {
		t.Fatal(err)
	}
	if done != 1 {
		t.Errorf("got done %d, want 1", done)
	}
	if carriedOver != 2 {
		t.Errorf("got carriedOver %d, want 2", carriedOver)
	}

	plan, _ := ps.GetPlan("2026-03-12")
	if plan.State != "reviewed" {
		t.Errorf("got state %q, want reviewed", plan.State)
	}
}

func TestInboxExcludesPlannedTasks(t *testing.T) {
	ts, ps, _ := setupStores(t)
	ts.Add("A")
	ts.Add("B")
	ts.Add("C")

	ps.CreatePlan("2026-03-12", []int64{1, 2})

	inbox, err := ts.InboxTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 1 {
		t.Errorf("got %d inbox, want 1 (only C)", len(inbox))
	}
	if inbox[0].ID != 3 {
		t.Errorf("got inbox task ID %d, want 3", inbox[0].ID)
	}
}

func TestAutoFixYesterday(t *testing.T) {
	ts, ps, _ := setupStores(t)
	ts.Add("昨日の残り")
	ps.CreatePlan("2026-03-11", []int64{1}) // yesterday, closed but not reviewed

	err := ps.AutoFixYesterday()
	if err != nil {
		t.Fatal(err)
	}

	plan, _ := ps.GetPlan("2026-03-11")
	if plan.State != "reviewed" {
		t.Errorf("got state %q, want reviewed", plan.State)
	}

	// Task should be back in inbox
	inbox, _ := ts.InboxTasks()
	if len(inbox) != 1 {
		t.Errorf("got %d inbox, want 1", len(inbox))
	}
}
