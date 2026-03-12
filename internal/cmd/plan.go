package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "朝の儀式: inboxから今日のWILL DOを選んでリストを閉じる",
		RunE: func(cmd *cobra.Command, args []string) error {
			today := time.Now().Format("2006-01-02")

			// Check if already planned today
			existing, err := planStore.GetPlan(today)
			if err != nil {
				return err
			}
			if existing != nil {
				return fmt.Errorf("今日 (%s) は既にplanしました（--replan で再計画）", today)
			}

			// Get inbox tasks
			inbox, err := taskStore.InboxTasks()
			if err != nil {
				return err
			}
			if len(inbox) == 0 {
				printLine("inboxが空です。まず todo add でタスクを追加してください。")
				return nil
			}

			// Show inbox
			printLine("📥 inbox (%d件):", len(inbox))
			for i, t := range inbox {
				printLine("  %d) #%d %s", i+1, t.ID, t.Title)
			}
			printLine("")
			printLine("今日やるタスクの番号をカンマ区切りで入力 (例: 1,3,5):")
			printLine("空Enter = 全部選択, q = キャンセル")

			// Read input
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			input := strings.TrimSpace(scanner.Text())

			if input == "q" || input == "Q" {
				printLine("キャンセルしました。")
				return nil
			}

			var selectedIDs []int64
			if input == "" {
				// Select all
				for _, t := range inbox {
					selectedIDs = append(selectedIDs, t.ID)
				}
			} else {
				// Parse numbers
				parts := strings.Split(input, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					n, err := strconv.Atoi(p)
					if err != nil || n < 1 || n > len(inbox) {
						return fmt.Errorf("無効な番号: %s (1-%d)", p, len(inbox))
					}
					selectedIDs = append(selectedIDs, inbox[n-1].ID)
				}
			}

			if len(selectedIDs) == 0 {
				printLine("タスクが選択されませんでした。")
				return nil
			}

			// Create plan
			if err := planStore.CreatePlan(today, selectedIDs); err != nil {
				return err
			}

			if flagJSON {
				items, _ := planStore.TodayItems(today)
				printJSON(items)
				return nil
			}

			printLine("")
			printLine("✅ 今日のWILL DO (%d件) — リスト閉鎖", len(selectedIDs))
			items, _ := planStore.TodayItems(today)
			for _, item := range items {
				printLine("  [ ] #%d %s", item.Task.ID, item.Task.Title)
			}
			return nil
		},
	}
}
