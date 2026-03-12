package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "今日のWILL DOリストを表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			today := time.Now().Format("2006-01-02")

			plan, err := planStore.GetPlan(today)
			if err != nil {
				return err
			}
			if plan == nil {
				printLine("今日はまだplanしていません。todo plan を実行してください。")
				return nil
			}

			items, err := planStore.TodayItems(today)
			if err != nil {
				return err
			}

			if flagJSON {
				printJSON(items)
				return nil
			}

			printLine("📋 WILL DO — %s", today)
			if len(items) == 0 {
				printLine("  (空)")
				return nil
			}
			for _, item := range items {
				check := "[ ]"
				if item.Disposition == "done" {
					check = "[✓]"
				}
				printLine("  %s #%d %s", check, item.Task.ID, item.Task.Title)
			}

			// Stats
			done := 0
			for _, item := range items {
				if item.Disposition == "done" {
					done++
				}
			}
			printLine("")
			printLine("  %d/%d 完了", done, len(items))
			return nil
		},
	}
}

func init() {
	_ = fmt.Sprintf
}
