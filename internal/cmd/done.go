package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "タスクを完了にする",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("無効なID: %s", args[0])
			}

			today := time.Now().Format("2006-01-02")

			// Check plan exists
			plan, err := planStore.GetPlan(today)
			if err != nil {
				return err
			}
			if plan == nil {
				return fmt.Errorf("今日はまだplanしていません")
			}

			// Mark done in tasks table
			task, err := taskStore.Done(id)
			if err != nil {
				return err
			}

			// Mark done in plan items
			if err := planStore.MarkDone(today, id); err != nil {
				return err
			}

			if flagJSON {
				printJSON(task)
				return nil
			}
			printLine("✓ #%d %s", task.ID, task.Title)
			return nil
		},
	}
}
