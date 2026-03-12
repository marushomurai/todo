package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [タスク名]",
		Short: "inboxにタスクを追加",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			task, err := taskStore.Add(title)
			if err != nil {
				return err
			}
			if flagJSON {
				printJSON(task)
				return nil
			}
			printLine("#%d %s → inbox", task.ID, task.Title)
			return nil
		},
	}
}

func init() {
	_ = fmt.Sprintf // avoid unused import
}
