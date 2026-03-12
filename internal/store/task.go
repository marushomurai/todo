package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/yuyanky/todo/internal/model"
)

type TaskStore struct {
	db *sql.DB
}

func NewTaskStore(db *sql.DB) *TaskStore {
	return &TaskStore{db: db}
}

func (s *TaskStore) Add(title string) (*model.Task, error) {
	res, err := s.db.Exec("INSERT INTO tasks (title) VALUES (?)", title)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	id, _ := res.LastInsertId()
	return &model.Task{
		ID:        id,
		Title:     title,
		Status:    model.StatusActive,
		CreatedAt: time.Now(),
	}, nil
}

func (s *TaskStore) Done(id int64) (*model.Task, error) {
	now := time.Now()
	res, err := s.db.Exec(
		"UPDATE tasks SET status = ?, done_at = ? WHERE id = ? AND status = ?",
		model.StatusDone, now, id, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("done task: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("task #%d not found or already done", id)
	}
	return s.Get(id)
}

func (s *TaskStore) Get(id int64) (*model.Task, error) {
	row := s.db.QueryRow("SELECT id, title, status, done_at, created_at FROM tasks WHERE id = ?", id)
	return scanTask(row)
}

func (s *TaskStore) ListActive() ([]*model.Task, error) {
	rows, err := s.db.Query("SELECT id, title, status, done_at, created_at FROM tasks WHERE status = ? ORDER BY id", model.StatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []*model.Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// InboxTasks returns active tasks NOT in any open/closed daily plan
func (s *TaskStore) InboxTasks() ([]*model.Task, error) {
	query := `
		SELECT t.id, t.title, t.status, t.done_at, t.created_at
		FROM tasks t
		WHERE t.status = ?
		  AND t.id NOT IN (
		    SELECT dpi.task_id FROM daily_plan_items dpi
		    JOIN daily_plans dp ON dp.plan_date = dpi.plan_date
		    WHERE dp.state IN ('open', 'closed')
		    AND dpi.disposition = 'planned'
		  )
		ORDER BY t.id`
	rows, err := s.db.Query(query, model.StatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []*model.Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func scanTask(row *sql.Row) (*model.Task, error) {
	var t model.Task
	var doneAt sql.NullTime
	if err := row.Scan(&t.ID, &t.Title, &t.Status, &doneAt, &t.CreatedAt); err != nil {
		return nil, err
	}
	if doneAt.Valid {
		t.DoneAt = &doneAt.Time
	}
	return &t, nil
}

func scanTaskRows(rows *sql.Rows) (*model.Task, error) {
	var t model.Task
	var doneAt sql.NullTime
	if err := rows.Scan(&t.ID, &t.Title, &t.Status, &doneAt, &t.CreatedAt); err != nil {
		return nil, err
	}
	if doneAt.Valid {
		t.DoneAt = &doneAt.Time
	}
	return &t, nil
}
