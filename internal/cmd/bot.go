package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	tgbot "github.com/yuyanky/todo/internal/telegram"
)

func newBotCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bot",
		Short: "Telegram Botを起動",
		RunE: func(cmd *cobra.Command, args []string) error {
			token := os.Getenv("TODO_TELEGRAM_TOKEN")
			if token == "" {
				// Try Keychain
				out, err := exec.Command("security", "find-generic-password", "-s", "todo-telegram", "-a", "bot_token", "-w").Output()
				if err != nil {
					return fmt.Errorf("TODO_TELEGRAM_TOKEN未設定、Keychainにも見つかりません")
				}
				token = strings.TrimSpace(string(out))
			}

			bot, err := tgbot.New(token, taskStore, planStore)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			fmt.Println("🤖 Telegram Bot起動中... (Ctrl+C で停止)")
			bot.Start(ctx)
			return nil
		},
	}
}
