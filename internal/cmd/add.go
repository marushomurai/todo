package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yuyanky/todo/internal/store"
)

func newAddCmd() *cobra.Command {
	var dueDate string
	cmd := &cobra.Command{
		Use:   "add [タスク名]",
		Short: "inboxにタスクを追加",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			task, err := taskStore.Add(title, store.AddOpts{DueDate: dueDate})
			if err != nil {
				return err
			}
			if flagJSON {
				printJSON(task)
				return nil
			}
			msg := fmt.Sprintf("#%d %s → inbox", task.ID, task.Title)
			if task.DueDate != "" {
				msg += fmt.Sprintf(" (期限: %s)", task.DueDate)
			}
			printLine("%s", msg)
			return nil
		},
	}
	cmd.Flags().StringVar(&dueDate, "due", "", "期限 (YYYY-MM-DD)")
	return cmd
}
