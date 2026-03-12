package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review",
		Short: "夕方の儀式: 未完了をinboxに戻して振り返り",
		RunE: func(cmd *cobra.Command, args []string) error {
			today := time.Now().Format("2006-01-02")

			plan, err := planStore.GetPlan(today)
			if err != nil {
				return err
			}
			if plan == nil {
				return fmt.Errorf("今日はまだplanしていません")
			}
			if plan.State == "reviewed" {
				return fmt.Errorf("今日は既にreviewしました")
			}

			// Show current state before review
			items, err := planStore.TodayItems(today)
			if err != nil {
				return err
			}

			done, carriedOver, err := planStore.Review(today)
			if err != nil {
				return err
			}
			total := done + carriedOver

			if flagJSON {
				printJSON(map[string]any{
					"date":         today,
					"total":        total,
					"done":         done,
					"carried_over": carriedOver,
					"rate":         fmt.Sprintf("%.0f%%", float64(done)/float64(total)*100),
				})
				return nil
			}

			printLine("📊 Review — %s", today)
			printLine("")
			for _, item := range items {
				if item.Disposition == "done" {
					printLine("  [✓] #%d %s", item.Task.ID, item.Task.Title)
				} else {
					printLine("  [→] #%d %s → inbox", item.Task.ID, item.Task.Title)
				}
			}
			printLine("")
			if total > 0 {
				rate := float64(done) / float64(total) * 100
				printLine("  完了: %d/%d (%.0f%%)", done, total, rate)
			}
			if carriedOver > 0 {
				printLine("  %d件をinboxに戻しました（明朝の再コミットを）", carriedOver)
			}
			return nil
		},
	}
}

func init() {
	_ = fmt.Sprintf
}
