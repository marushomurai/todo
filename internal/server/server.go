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
	"github.com/yuyanky/todo/internal/model"
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

func greeting() string {
	h := time.Now().Hour()
	switch {
	case h >= 5 && h < 12:
		return "おはようございます"
	case h >= 12 && h < 18:
		return "こんにちは"
	default:
		return "おつかれさまです"
	}
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
	s.mux.HandleFunc("POST /api/move-today/{id}", s.handleAPIMoveToday)
	s.mux.HandleFunc("GET /api/inbox-edit/{id}", s.handleAPIInboxEdit)
	s.mux.HandleFunc("POST /api/inbox-edit/{id}", s.handleAPIInboxEditSave)
	s.mux.HandleFunc("GET /api/inbox-item/{id}", s.handleAPIInboxItem)
	s.mux.HandleFunc("POST /api/remove-today/{id}", s.handleAPIRemoveToday)
	s.mux.HandleFunc("GET /api/today-edit/{id}", s.handleAPITodayEdit)
	s.mux.HandleFunc("POST /api/today-edit/{id}", s.handleAPITodayEditSave)
	s.mux.HandleFunc("GET /api/today-item/{id}", s.handleAPITodayItem)
}

func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	date := today()
	plan, _ := s.plans.GetPlan(date)

	data := view.TodayData{
		Date:     date,
		HasPlan:  plan != nil,
		Greeting: greeting(),
	}

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
	} else {
		// No plan yet — load inbox for plan selection
		tasks, _ := s.tasks.InboxTasks()
		data.InboxTasks = tasks
	}
	render(w, r, view.Today(data))
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	tasks, _ := s.tasks.InboxTasks()
	plan, _ := s.plans.GetPlan(today())
	render(w, r, view.Inbox(view.InboxData{Tasks: tasks, HasPlan: plan != nil}))
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
	plan, _ := s.plans.GetPlan(today())
	render(w, r, view.InboxRow(task, plan != nil))
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
			render(w, r, view.TodoRowWithProgress(item, done, total))
			return
		}
	}
}

func (s *Server) handleAPIPlan(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ids := r.Form["task_ids"]
	if len(ids) == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
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
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleAPIReview(w http.ResponseWriter, r *http.Request) {
	date := today()
	if _, _, err := s.plans.Review(date); err != nil {
		slog.Error("review", "err", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
	requestedBy := strings.TrimSpace(r.FormValue("requested_by"))

	_, err = s.tasks.Update(id, title, dueDate, notes, requestedBy)
	if err != nil {
		slog.Error("update task", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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

	items, _ := s.plans.TodayItems(date)
	done, total := 0, len(items)
	for _, item := range items {
		if item.Disposition == "done" {
			done++
		}
	}
	for _, item := range items {
		if item.TaskID == task.ID {
			render(w, r, view.TodoRowWithProgress(item, done, total))
			return
		}
	}
}

func (s *Server) handleAPIInboxEdit(w http.ResponseWriter, r *http.Request) {
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
	plan, _ := s.plans.GetPlan(today())
	render(w, r, view.InboxRowEdit(task, plan != nil))
}

func (s *Server) handleAPIInboxEditSave(w http.ResponseWriter, r *http.Request) {
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
	task, err := s.tasks.Get(id)
	if err != nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	task, err = s.tasks.Update(id, title, task.DueDate, task.Notes, task.RequestedBy)
	if err != nil {
		slog.Error("update task title", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	plan, _ := s.plans.GetPlan(today())
	render(w, r, view.InboxRow(task, plan != nil))
}

func (s *Server) handleAPIInboxItem(w http.ResponseWriter, r *http.Request) {
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
	plan, _ := s.plans.GetPlan(today())
	render(w, r, view.InboxRow(task, plan != nil))
}

func (s *Server) handleAPIMoveToday(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	date := today()
	if err := s.plans.AddToTodayPlan(date, id); err != nil {
		slog.Error("move to today", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
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
	if r.Header.Get("HX-Request") != "true" {
		back := r.URL.Query().Get("back")
		if back == "" {
			back = "/"
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleAPIRemoveToday(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	date := today()
	if err := s.plans.RemoveFromTodayPlan(date, id); err != nil {
		slog.Error("remove from today", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items, _ := s.plans.TodayItems(date)
	done, total := 0, len(items)
	for _, item := range items {
		if item.Disposition == "done" {
			done++
		}
	}
	render(w, r, view.ProgressUpdatesOnly(done, total))
}

func (s *Server) handleAPITodayEdit(w http.ResponseWriter, r *http.Request) {
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
	date := today()
	items, _ := s.plans.TodayItems(date)
	for _, item := range items {
		if item.TaskID == id {
			render(w, r, view.TodoRowEdit(item))
			return
		}
	}
	render(w, r, view.TodoRowEdit(model.PlanItem{
		TaskID:      task.ID,
		Disposition: "planned",
		Task:        *task,
	}))
}

func (s *Server) handleAPITodayEditSave(w http.ResponseWriter, r *http.Request) {
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
	task, err := s.tasks.Get(id)
	if err != nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if _, err = s.tasks.Update(id, title, task.DueDate, task.Notes, task.RequestedBy); err != nil {
		slog.Error("update task title", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	date := today()
	items, _ := s.plans.TodayItems(date)
	for _, item := range items {
		if item.TaskID == id {
			render(w, r, view.TodoRow(item))
			return
		}
	}
}

func (s *Server) handleAPITodayItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	date := today()
	items, _ := s.plans.TodayItems(date)
	for _, item := range items {
		if item.TaskID == id {
			render(w, r, view.TodoRow(item))
			return
		}
	}
	http.Error(w, "item not found", http.StatusNotFound)
}

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(context.Background(), w); err != nil {
		slog.Error("render", "err", err)
		fmt.Fprintf(w, "render error: %v", err)
	}
}
