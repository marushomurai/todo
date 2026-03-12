package store_test

import (
	"testing"

	"github.com/yuyanky/todo/internal/db"
	"github.com/yuyanky/todo/internal/model"
	"github.com/yuyanky/todo/internal/store"
)

func setupDB(t *testing.T) *store.TaskStore {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return store.NewTaskStore(d)
}

func TestAdd(t *testing.T) {
	s := setupDB(t)
	task, err := s.Add("買い物")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != 1 {
		t.Errorf("got ID %d, want 1", task.ID)
	}
	if task.Title != "買い物" {
		t.Errorf("got Title %q, want 買い物", task.Title)
	}
	if task.Status != model.StatusActive {
		t.Errorf("got Status %q, want active", task.Status)
	}
}

func TestDone(t *testing.T) {
	s := setupDB(t)
	task, _ := s.Add("テスト")
	done, err := s.Done(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != model.StatusDone {
		t.Errorf("got Status %q, want done", done.Status)
	}
	if done.DoneAt == nil {
		t.Error("DoneAt should not be nil")
	}
}

func TestDone_AlreadyDone(t *testing.T) {
	s := setupDB(t)
	task, _ := s.Add("テスト")
	s.Done(task.ID)
	_, err := s.Done(task.ID)
	if err == nil {
		t.Error("expected error for double done")
	}
}

func TestDone_NotFound(t *testing.T) {
	s := setupDB(t)
	_, err := s.Done(999)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestListActive(t *testing.T) {
	s := setupDB(t)
	s.Add("A")
	s.Add("B")
	s.Add("C")
	s.Done(2)

	tasks, err := s.ListActive()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("got %d active, want 2", len(tasks))
	}
}

func TestInboxTasks(t *testing.T) {
	s := setupDB(t)
	s.Add("A")
	s.Add("B")
	s.Add("C")

	inbox, err := s.InboxTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 3 {
		t.Errorf("got %d inbox, want 3", len(inbox))
	}
}
