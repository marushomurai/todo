package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/yuyanky/todo/internal/store"
	"github.com/yuyanky/todo/internal/view"
)

type Server struct {
	tasks *store.TaskStore
	plans *store.PlanStore
	mux   *http.ServeMux
}

func New(tasks *store.TaskStore, plans *store.PlanStore, staticDir string) *Server {
	s := &Server{tasks: tasks, plans: plans, mux: http.NewServeMux()}
	s.routes(staticDir)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func (s *Server) routes(staticDir string) {
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	s.mux.HandleFunc("GET /{$}", s.handleToday)
	s.mux.HandleFunc("GET /inbox", s.handleInbox)
	s.mux.HandleFunc("GET /plan", s.handlePlan)
	s.mux.HandleFunc("GET /review", s.handleReview)

	s.mux.HandleFunc("GET /task/{id}", s.handleDetail)

	s.mux.HandleFunc("POST /api/add", s.handleAPIAdd)
	s.mux.HandleFunc("POST /api/task/{id}", s.handleAPIUpdateTask)
	s.mux.HandleFunc("POST /api/done/{id}", s.handleAPIDone)
	s.mux.HandleFunc("POST /api/undone/{id}", s.handleAPIUndone)
	s.mux.HandleFunc("POST /api/plan", s.handleAPIPlan)
	s.mux.HandleFunc("POST /api/review", s.handleAPIReview)
	s.mux.HandleFunc("POST /api/reorder", s.handleAPIReorder)
	s.mux.HandleFunc("POST /api/reorder-inbox", s.handleAPIReorderInbox)
	s.mux.HandleFunc("POST /api/delete/{id}", s.handleAPIDelete)
	s.mux.HandleFunc("POST /api/add-today", s.handleAPIAddToday)
}

func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	date := today()
	plan, _ := s.plans.GetPlan(date)

	data := view.TodayData{Date: date, HasPlan: plan != nil}
	if plan != nil {
		items, _ := s.plans.TodayItems(date)
		data.Items = items
		data.Total = len(items)
		for _, item := range items {
			if item.Disposition == "done" {
				data.Done++
			}
		}
	}
	render(w, r, view.Today(data))
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	tasks, _ := s.tasks.InboxTasks()
	render(w, r, view.Inbox(view.InboxData{Tasks: tasks}))
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	date := today()
	plan, _ := s.plans.GetPlan(date)
	if plan != nil {
		render(w, r, view.Plan(view.PlanData{AlreadyDone: true, Date: date}))
		return
	}
	tasks, _ := s.tasks.InboxTasks()
	render(w, r, view.Plan(view.PlanData{Tasks: tasks}))
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	date := today()
	plan, _ := s.plans.GetPlan(date)

	data := view.ReviewData{Date: date, HasPlan: plan != nil}
	if plan != nil {
		items, _ := s.plans.TodayItems(date)
		data.Items = items
		data.IsReviewed = plan.State == "reviewed"
		data.Total = len(items)
		for _, item := range items {
			if item.Disposition == "done" {
				data.Done++
			} else if item.Disposition == "carried_over" {
				data.CarriedOver++
			}
		}
	}
	render(w, r, view.Review(data))
}

func (s *Server) handleAPIAdd(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	dueDate := strings.TrimSpace(r.FormValue("due_date"))
	task, err := s.tasks.Add(title, store.AddOpts{DueDate: dueDate})
	if err != nil {
		slog.Error("add task", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, view.InboxItem(task))
}

// handleAPIDone + OOB progress bar update
func (s *Server) handleAPIDone(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	date := today()
	if _, err := s.tasks.Done(id); err != nil {
		slog.Error("done task", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.plans.MarkDone(date, id); err != nil {
		slog.Error("mark done in plan", "err", err)
	}

	s.renderItemWithProgress(w, r, date, id)
}

// handleAPIUndone reverts a done task back to planned
func (s *Server) handleAPIUndone(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	date := today()
	if _, err := s.tasks.Undone(id); err != nil {
		slog.Error("undone task", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.plans.UnMarkDone(date, id); err != nil {
		slog.Error("unmark done in plan", "err", err)
	}

	s.renderItemWithProgress(w, r, date, id)
}

// renderItemWithProgress returns the updated item + OOB progress bar
func (s *Server) renderItemWithProgress(w http.ResponseWriter, r *http.Request, date string, id int64) {
	items, _ := s.plans.TodayItems(date)
	done, total := 0, len(items)
	for _, item := range items {
		if item.Disposition == "done" {
			done++
		}
	}

	for _, item := range items {
		if item.TaskID == id {
			render(w, r, view.TodoItemWithProgress(item, done, total))
			return
		}
	}
}

func (s *Server) handleAPIPlan(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ids := r.Form["task_ids"]
	if len(ids) == 0 {
		http.Redirect(w, r, "/plan", http.StatusSeeOther)
		return
	}
	var taskIDs []int64
	for _, idStr := range ids {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		taskIDs = append(taskIDs, id)
	}

	date := today()
	if err := s.plans.CreatePlan(date, taskIDs); err != nil {
		slog.Error("create plan", "err", err)
		http.Redirect(w, r, "/plan", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleAPIReview(w http.ResponseWriter, r *http.Request) {
	date := today()
	if _, _, err := s.plans.Review(date); err != nil {
		slog.Error("review", "err", err)
	}
	http.Redirect(w, r, "/review", http.StatusSeeOther)
}

func (s *Server) handleAPIReorder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TaskIDs []int64 `json:"task_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	date := today()
	if err := s.plans.Reorder(date, body.TaskIDs); err != nil {
		slog.Error("reorder", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	task, err := s.tasks.Get(id)
	if err != nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	back := r.Referer()
	if back == "" {
		back = "/"
	}
	render(w, r, view.Detail(view.DetailData{Task: task, Back: back}))
}

func (s *Server) handleAPIUpdateTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	dueDate := strings.TrimSpace(r.FormValue("due_date"))
	notes := r.FormValue("notes")

	_, err = s.tasks.Update(id, title, dueDate, notes)
	if err != nil {
		slog.Error("update task", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Return to the referring page, default to /
	back := r.FormValue("back")
	if back == "" {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) handleAPIReorderInbox(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TaskIDs []int64 `json:"task_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.tasks.ReorderInbox(body.TaskIDs); err != nil {
		slog.Error("reorder inbox", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIAddToday(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	dueDate := strings.TrimSpace(r.FormValue("due_date"))
	task, err := s.tasks.Add(title, store.AddOpts{DueDate: dueDate})
	if err != nil {
		slog.Error("add task", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	date := today()
	if err := s.plans.AddToTodayPlan(date, task.ID); err != nil {
		slog.Error("add to today plan", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the new item + updated progress OOB
	items, _ := s.plans.TodayItems(date)
	done, total := 0, len(items)
	for _, item := range items {
		if item.Disposition == "done" {
			done++
		}
	}
	for _, item := range items {
		if item.TaskID == task.ID {
			render(w, r, view.TodoItemWithProgress(item, done, total))
			return
		}
	}
}

func (s *Server) handleAPIDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.tasks.Delete(id); err != nil {
		slog.Error("delete task", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// If called from detail page (non-HTMX), redirect back
	if r.Header.Get("HX-Request") != "true" {
		back := r.URL.Query().Get("back")
		if back == "" {
			back = "/"
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	// HTMX: return empty to remove element
	w.WriteHeader(http.StatusOK)
}

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(context.Background(), w); err != nil {
		slog.Error("render", "err", err)
		fmt.Fprintf(w, "render error: %v", err)
	}
}
