package model

import "time"

type Status string

const (
	StatusActive    Status = "active"
	StatusDone      Status = "done"
	StatusCancelled Status = "cancelled"
)

type PlanState string

const (
	PlanOpen     PlanState = "open"
	PlanClosed   PlanState = "closed"
	PlanReviewed PlanState = "reviewed"
)

type Disposition string

const (
	DispositionPlanned    Disposition = "planned"
	DispositionDone       Disposition = "done"
	DispositionCarriedOver Disposition = "carried_over"
)

type Task struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	Status        Status     `json:"status"`
	DoneAt        *time.Time `json:"done_at,omitempty"`
	DueDate       string     `json:"due_date,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	InboxPosition int        `json:"inbox_position"`
	CreatedAt     time.Time  `json:"created_at"`
}

type DailyPlan struct {
	PlanDate   string     `json:"plan_date"`
	State      PlanState  `json:"state"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type DailyPlanItem struct {
	PlanDate    string      `json:"plan_date"`
	TaskID      int64       `json:"task_id"`
	Position    int         `json:"position"`
	Disposition Disposition `json:"disposition"`
}

type PlanItem struct {
	PlanDate    string      `json:"plan_date"`
	TaskID      int64       `json:"task_id"`
	Position    int         `json:"position"`
	Disposition Disposition `json:"disposition"`
	Task        Task        `json:"task"`
}
