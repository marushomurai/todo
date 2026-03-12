package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yuyanky/todo/internal/server"
)

func newServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Web UIを起動",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find static dir relative to executable or CWD
			staticDir := findStaticDir()
			if staticDir == "" {
				return fmt.Errorf("static/ directory not found")
			}

			srv := server.New(taskStore, planStore, staticDir)
			addr := fmt.Sprintf(":%d", port)
			slog.Info("starting server", "addr", addr, "static", staticDir)
			fmt.Printf("🌐 http://localhost:%d\n", port)
			return http.ListenAndServe(addr, srv)
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 3456, "ポート番号")
	return cmd
}

func findStaticDir() string {
	// Check CWD first
	if info, err := os.Stat("static"); err == nil && info.IsDir() {
		return "static"
	}
	// Check relative to executable
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Join(filepath.Dir(exe), "static")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}
