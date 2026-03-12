package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yuyanky/todo/internal/db"
	"github.com/yuyanky/todo/internal/store"
)

var (
	flagJSON    bool
	flagQuiet   bool
	flagNoColor bool

	database   *sql.DB
	taskStore  *store.TaskStore
	planStore  *store.PlanStore
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "todo",
		Short: "マニャーナの法則 — 日次コミットメント装置",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			database, err = db.Open(db.DefaultPath())
			if err != nil {
				return err
			}
			taskStore = store.NewTaskStore(database)
			planStore = store.NewPlanStore(database)

			// Auto-fix unreviewed past plans
			return planStore.AutoFixYesterday()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if database != nil {
				database.Close()
			}
		},
		SilenceUsage: true,
	}

	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "JSON出力")
	root.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "出力抑制")
	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "カラー無効")

	// Respect NO_COLOR env
	if os.Getenv("NO_COLOR") != "" {
		flagNoColor = true
	}

	root.AddCommand(newAddCmd())
	root.AddCommand(newPlanCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newDoneCmd())
	root.AddCommand(newReviewCmd())

	return root
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func printLine(format string, a ...any) {
	if !flagQuiet {
		fmt.Printf(format+"\n", a...)
	}
}
