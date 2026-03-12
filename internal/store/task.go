package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/yuyanky/todo/internal/model"
)

const taskCols = "id, title, status, done_at, due_date, notes, inbox_position, created_at"

type TaskStore struct {
	db *sql.DB
}

func NewTaskStore(db *sql.DB) *TaskStore {
	return &TaskStore{db: db}
}

type AddOpts struct {
	DueDate string
	Notes   string
}

func (s *TaskStore) Add(title string, opts ...AddOpts) (*model.Task, error) {
	var o AddOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	var dueVal any
	if o.DueDate != "" {
		dueVal = o.DueDate
	}
	res, err := s.db.Exec(
		"INSERT INTO tasks (title, due_date, notes) VALUES (?, ?, ?)",
		title, dueVal, o.Notes,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	id, _ := res.LastInsertId()
	s.db.Exec("UPDATE tasks SET inbox_position = ? WHERE id = ?", id, id)
	return s.Get(id)
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

func (s *TaskStore) Undone(id int64) (*model.Task, error) {
	res, err := s.db.Exec(
		"UPDATE tasks SET status = ?, done_at = NULL WHERE id = ? AND status = ?",
		model.StatusActive, id, model.StatusDone,
	)
	if err != nil {
		return nil, fmt.Errorf("undone task: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("task #%d not found or not done", id)
	}
	return s.Get(id)
}

func (s *TaskStore) Delete(id int64) error {
	// Remove from any plan items first
	s.db.Exec("DELETE FROM daily_plan_items WHERE task_id = ?", id)
	res, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task #%d not found", id)
	}
	return nil
}

func (s *TaskStore) Update(id int64, title, dueDate, notes string) (*model.Task, error) {
	var dueDateVal any
	if dueDate == "" {
		dueDateVal = nil
	} else {
		dueDateVal = dueDate
	}
	_, err := s.db.Exec(
		"UPDATE tasks SET title = ?, due_date = ?, notes = ? WHERE id = ?",
		title, dueDateVal, notes, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	return s.Get(id)
}

func (s *TaskStore) Get(id int64) (*model.Task, error) {
	row := s.db.QueryRow("SELECT "+taskCols+" FROM tasks WHERE id = ?", id)
	return scanTask(row)
}

func (s *TaskStore) ListActive() ([]*model.Task, error) {
	rows, err := s.db.Query("SELECT "+taskCols+" FROM tasks WHERE status = ? ORDER BY id", model.StatusActive)
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
		SELECT t.id, t.title, t.status, t.done_at, t.due_date, t.notes, t.inbox_position, t.created_at
		FROM tasks t
		WHERE t.status = ?
		  AND t.id NOT IN (
		    SELECT dpi.task_id FROM daily_plan_items dpi
		    JOIN daily_plans dp ON dp.plan_date = dpi.plan_date
		    WHERE dp.state IN ('open', 'closed')
		    AND dpi.disposition = 'planned'
		  )
		ORDER BY CASE WHEN t.inbox_position = 0 THEN t.id ELSE t.inbox_position END`
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

func (s *TaskStore) ReorderInbox(taskIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, tid := range taskIDs {
		if _, err := tx.Exec("UPDATE tasks SET inbox_position = ? WHERE id = ?", i+1, tid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanTask(row *sql.Row) (*model.Task, error) {
	var t model.Task
	var doneAt sql.NullTime
	var dueDate sql.NullString
	if err := row.Scan(&t.ID, &t.Title, &t.Status, &doneAt, &dueDate, &t.Notes, &t.InboxPosition, &t.CreatedAt); err != nil {
		return nil, err
	}
	if doneAt.Valid {
		t.DoneAt = &doneAt.Time
	}
	if dueDate.Valid {
		t.DueDate = dueDate.String
	}
	return &t, nil
}

func scanTaskRows(rows *sql.Rows) (*model.Task, error) {
	var t model.Task
	var doneAt sql.NullTime
	var dueDate sql.NullString
	if err := rows.Scan(&t.ID, &t.Title, &t.Status, &doneAt, &dueDate, &t.Notes, &t.InboxPosition, &t.CreatedAt); err != nil {
		return nil, err
	}
	if doneAt.Valid {
		t.DoneAt = &doneAt.Time
	}
	if dueDate.Valid {
		t.DueDate = dueDate.String
	}
	return &t, nil
}
