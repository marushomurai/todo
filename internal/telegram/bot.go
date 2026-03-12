package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/yuyanky/todo/internal/store"
)

type Bot struct {
	tasks *store.TaskStore
	plans *store.PlanStore
	b     *bot.Bot
}

func New(token string, tasks *store.TaskStore, plans *store.PlanStore) (*Bot, error) {
	tb := &Bot{tasks: tasks, plans: plans}

	opts := []bot.Option{
		bot.WithDefaultHandler(tb.handleDefault),
	}
	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}
	tb.b = b

	b.RegisterHandler(bot.HandlerTypeMessageText, "/ls", bot.MatchTypePrefix, tb.handleLs)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/inbox", bot.MatchTypePrefix, tb.handleInbox)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/done", bot.MatchTypePrefix, tb.handleDone)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/add", bot.MatchTypePrefix, tb.handleAdd)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypePrefix, tb.handleStart)

	return tb, nil
}

func (tb *Bot) Start(ctx context.Context) {
	slog.Info("telegram bot starting")
	tb.b.Start(ctx)
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func (tb *Bot) reply(ctx context.Context, chatID int64, text string) {
	tb.b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
}

// parseDateSuffix extracts a trailing date pattern from text.
// Supports: MMDD, MDD, M/DD, MM/DD
// e.g. "テスト 0325" → ("テスト", "2026-03-25")
//      "牛乳 3/25"   → ("牛乳", "2026-03-25")
var dateSuffixRe = regexp.MustCompile(`\s+((\d{1,2})/(\d{1,2})|(\d{3,4}))$`)

func parseDateSuffix(text string) (title string, dueDate string) {
	m := dateSuffixRe.FindStringSubmatchIndex(text)
	if m == nil {
		return text, ""
	}
	title = strings.TrimSpace(text[:m[0]])
	datePart := strings.TrimSpace(text[m[0]:])

	var month, day int
	if strings.Contains(datePart, "/") {
		parts := strings.Split(datePart, "/")
		month, _ = strconv.Atoi(parts[0])
		day, _ = strconv.Atoi(parts[1])
	} else {
		n, _ := strconv.Atoi(datePart)
		if n >= 100 {
			// MMDD or MDD: last 2 digits = day, rest = month
			day = n % 100
			month = n / 100
		} else {
			return text, ""
		}
	}

	if month < 1 || month > 12 || day < 1 || day > 31 {
		return text, ""
	}

	year := time.Now().Year()
	// If the date is in the past, assume next year
	candidate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	if candidate.Before(time.Now().Truncate(24 * time.Hour)) {
		year++
	}
	dueDate = fmt.Sprintf("%d-%02d-%02d", year, month, day)
	return title, dueDate
}

// splitTitleNotes splits multi-line text into title (first line) and notes (rest).
func splitTitleNotes(text string) (title, notes string) {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		title = strings.TrimSpace(text[:i])
		notes = strings.TrimSpace(text[i+1:])
	} else {
		title = text
	}
	return
}

// autoEmoji prepends a contextual emoji if the title doesn't already start with one.
func autoEmoji(title string) string {
	if title == "" {
		return title
	}
	// Already starts with emoji? skip
	r, _ := utf8.DecodeRuneInString(title)
	if r >= 0x1F000 {
		return title
	}

	lower := strings.ToLower(title)
	emojiMap := []struct {
		keywords []string
		emoji    string
	}{
		{[]string{"買", "buy", "購入", "注文"}, "🛒"},
		{[]string{"牛乳", "卵", "パン", "米", "食", "ご飯", "ランチ", "dinner", "lunch"}, "🍽️"},
		{[]string{"電話", "call", "連絡", "tel"}, "📞"},
		{[]string{"メール", "mail", "email", "返信"}, "📧"},
		{[]string{"会議", "mtg", "meeting", "ミーティング", "打合", "打ち合わせ"}, "📅"},
		{[]string{"修正", "fix", "bug", "バグ", "直す"}, "🔧"},
		{[]string{"書く", "write", "記事", "ドキュメント", "doc", "資料"}, "📝"},
		{[]string{"読む", "read", "本", "book"}, "📖"},
		{[]string{"掃除", "clean", "片付", "整理"}, "🧹"},
		{[]string{"運動", "gym", "walk", "散歩", "筋トレ"}, "💪"},
		{[]string{"医者", "病院", "歯医者", "薬"}, "🏥"},
		{[]string{"送る", "send", "郵送", "発送", "配送"}, "📦"},
		{[]string{"支払", "pay", "振込", "入金"}, "💰"},
		{[]string{"予約", "reserve", "booking"}, "🎫"},
		{[]string{"チェック", "check", "確認", "review", "レビュー"}, "✅"},
		{[]string{"調べ", "research", "調査", "search"}, "🔍"},
		{[]string{"作る", "create", "作成", "build", "実装"}, "🔨"},
		{[]string{"デザイン", "design"}, "🎨"},
	}
	for _, e := range emojiMap {
		for _, kw := range e.keywords {
			if strings.Contains(lower, kw) {
				return e.emoji + " " + title
			}
		}
	}
	return "📌 " + title
}

// handleDefault: plain text → inbox add
func (tb *Bot) handleDefault(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}
	text := strings.TrimSpace(update.Message.Text)
	if strings.HasPrefix(text, "/") {
		return // unknown command, ignore
	}

	firstLine, notes := splitTitleNotes(text)
	title, dueDate := parseDateSuffix(firstLine)
	title = autoEmoji(title)
	task, err := tb.tasks.Add(title, store.AddOpts{DueDate: dueDate, Notes: notes})
	if err != nil {
		tb.reply(ctx, update.Message.Chat.ID, "❌ "+err.Error())
		return
	}
	msg := fmt.Sprintf("📥 #%d *%s* → inbox", task.ID, task.Title)
	if task.DueDate != "" {
		msg += fmt.Sprintf(" (期限: %s)", task.DueDate)
	}
	if task.Notes != "" {
		msg += "\n📎 メモ付き"
	}
	tb.reply(ctx, update.Message.Chat.ID, msg)
}

func (tb *Bot) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	tb.reply(ctx, update.Message.Chat.ID,
		"*mañana* — 日次コミットメント装置\n\n"+
			"テキスト送信 → inbox追加\n"+
			"/ls — 今日のWILL DO\n"+
			"/inbox — inbox一覧\n"+
			"/done 3 — タスク完了\n"+
			"/add 牛乳買う — inbox追加")
}

func (tb *Bot) handleAdd(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/add"))
	if text == "" {
		tb.reply(ctx, update.Message.Chat.ID, "使い方: /add タスク名\n日付付き: /add タスク名 0325\n改行でメモ付き")
		return
	}
	firstLine, notes := splitTitleNotes(text)
	title, dueDate := parseDateSuffix(firstLine)
	title = autoEmoji(title)
	task, err := tb.tasks.Add(title, store.AddOpts{DueDate: dueDate, Notes: notes})
	if err != nil {
		tb.reply(ctx, update.Message.Chat.ID, "❌ "+err.Error())
		return
	}
	msg := fmt.Sprintf("📥 #%d *%s* → inbox", task.ID, task.Title)
	if task.DueDate != "" {
		msg += fmt.Sprintf(" (期限: %s)", task.DueDate)
	}
	if task.Notes != "" {
		msg += "\n📎 メモ付き"
	}
	tb.reply(ctx, update.Message.Chat.ID, msg)
}

func (tb *Bot) handleLs(ctx context.Context, b *bot.Bot, update *models.Update) {
	date := today()
	plan, _ := tb.plans.GetPlan(date)
	if plan == nil {
		tb.reply(ctx, update.Message.Chat.ID, "まだ今日のplanがありません")
		return
	}

	items, _ := tb.plans.TodayItems(date)
	if len(items) == 0 {
		tb.reply(ctx, update.Message.Chat.ID, "WILL DOは空です")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*WILL DO — %s*\n\n", date))
	done := 0
	for _, item := range items {
		if item.Disposition == "done" {
			sb.WriteString(fmt.Sprintf("✅ ~#%d %s~\n", item.Task.ID, item.Task.Title))
			done++
		} else {
			sb.WriteString(fmt.Sprintf("⬜ #%d %s\n", item.Task.ID, item.Task.Title))
		}
	}
	sb.WriteString(fmt.Sprintf("\n%d/%d 完了", done, len(items)))
	tb.reply(ctx, update.Message.Chat.ID, sb.String())
}

func (tb *Bot) handleInbox(ctx context.Context, b *bot.Bot, update *models.Update) {
	tasks, _ := tb.tasks.InboxTasks()
	if len(tasks) == 0 {
		tb.reply(ctx, update.Message.Chat.ID, "inboxは空です")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*📥 inbox (%d件)*\n\n", len(tasks)))
	for _, t := range tasks {
		line := fmt.Sprintf("• #%d %s", t.ID, t.Title)
		if t.DueDate != "" {
			line += fmt.Sprintf(" (%s)", t.DueDate)
		}
		sb.WriteString(line + "\n")
	}
	tb.reply(ctx, update.Message.Chat.ID, sb.String())
}

func (tb *Bot) handleDone(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/done"))
	if text == "" {
		tb.reply(ctx, update.Message.Chat.ID, "使い方: /done タスクID")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		tb.reply(ctx, update.Message.Chat.ID, "無効なID: "+text)
		return
	}

	task, err := tb.tasks.Done(id)
	if err != nil {
		tb.reply(ctx, update.Message.Chat.ID, "❌ "+err.Error())
		return
	}

	// Also mark done in today's plan if applicable
	date := today()
	tb.plans.MarkDone(date, id)

	tb.reply(ctx, update.Message.Chat.ID, fmt.Sprintf("✅ #%d *%s*", task.ID, task.Title))
}
