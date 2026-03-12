package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/yuyanky/todo/internal/model"
)

type PlanStore struct {
	db *sql.DB
}

func NewPlanStore(db *sql.DB) *PlanStore {
	return &PlanStore{db: db}
}

func today() string {
	return time.Now().Format("2006-01-02")
}

// GetPlan returns the plan for a given date, or nil if none exists.
func (s *PlanStore) GetPlan(date string) (*model.DailyPlan, error) {
	row := s.db.QueryRow("SELECT plan_date, state, closed_at, reviewed_at, created_at FROM daily_plans WHERE plan_date = ?", date)
	var p model.DailyPlan
	var closedAt, reviewedAt sql.NullTime
	if err := row.Scan(&p.PlanDate, &p.State, &closedAt, &reviewedAt, &p.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if closedAt.Valid {
		p.ClosedAt = &closedAt.Time
	}
	if reviewedAt.Valid {
		p.ReviewedAt = &reviewedAt.Time
	}
	return &p, nil
}

// CreatePlan creates today's plan with selected task IDs, and closes it.
func (s *PlanStore) CreatePlan(date string, taskIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	_, err = tx.Exec(
		"INSERT INTO daily_plans (plan_date, state, closed_at) VALUES (?, 'closed', ?)",
		date, now,
	)
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}

	for i, tid := range taskIDs {
		_, err = tx.Exec(
			"INSERT INTO daily_plan_items (plan_date, task_id, position) VALUES (?, ?, ?)",
			date, tid, i+1,
		)
		if err != nil {
			return fmt.Errorf("add plan item: %w", err)
		}
	}
	return tx.Commit()
}

// TodayItems returns the WILL DO items for the given date with task details.
func (s *PlanStore) TodayItems(date string) ([]model.PlanItem, error) {
	query := `
		SELECT dpi.plan_date, dpi.task_id, dpi.position, dpi.disposition,
		       t.id, t.title, t.status, t.done_at, t.created_at
		FROM daily_plan_items dpi
		JOIN tasks t ON t.id = dpi.task_id
		WHERE dpi.plan_date = ?
		ORDER BY dpi.position`
	rows, err := s.db.Query(query, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.PlanItem
	for rows.Next() {
		var pi model.PlanItem
		var doneAt sql.NullTime
		if err := rows.Scan(
			&pi.PlanDate, &pi.TaskID, &pi.Position, &pi.Disposition,
			&pi.Task.ID, &pi.Task.Title, &pi.Task.Status, &doneAt, &pi.Task.CreatedAt,
		); err != nil {
			return nil, err
		}
		if doneAt.Valid {
			pi.Task.DoneAt = &doneAt.Time
		}
		items = append(items, pi)
	}
	return items, rows.Err()
}

// MarkDone marks a plan item as done.
func (s *PlanStore) MarkDone(date string, taskID int64) error {
	res, err := s.db.Exec(
		"UPDATE daily_plan_items SET disposition = 'done' WHERE plan_date = ? AND task_id = ?",
		date, taskID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task #%d not in today's plan", taskID)
	}
	return nil
}

// Review closes out today's plan: marks as reviewed, carries over unfinished items.
func (s *PlanStore) Review(date string) (done int, carriedOver int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// Count done
	row := tx.QueryRow("SELECT COUNT(*) FROM daily_plan_items WHERE plan_date = ? AND disposition = 'done'", date)
	if err := row.Scan(&done); err != nil {
		return 0, 0, err
	}

	// Carry over planned (unfinished) items
	res, err := tx.Exec(
		"UPDATE daily_plan_items SET disposition = 'carried_over' WHERE plan_date = ? AND disposition = 'planned'",
		date,
	)
	if err != nil {
		return 0, 0, err
	}
	n, _ := res.RowsAffected()
	carriedOver = int(n)

	// Mark plan as reviewed
	now := time.Now()
	_, err = tx.Exec(
		"UPDATE daily_plans SET state = 'reviewed', reviewed_at = ? WHERE plan_date = ?",
		now, date,
	)
	if err != nil {
		return 0, 0, err
	}

	return done, carriedOver, tx.Commit()
}

// AutoFixYesterday carries over any unreviewed past plans' items.
func (s *PlanStore) AutoFixYesterday() error {
	_, err := s.db.Exec(`
		UPDATE daily_plan_items SET disposition = 'carried_over'
		WHERE disposition = 'planned'
		  AND plan_date < ?
		  AND plan_date IN (SELECT plan_date FROM daily_plans WHERE state = 'closed')`,
		today(),
	)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE daily_plans SET state = 'reviewed', reviewed_at = datetime('now', 'localtime')
		WHERE state = 'closed' AND plan_date < ?`,
		today(),
	)
	return err
}
