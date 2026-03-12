package main

import (
	"os"

	"github.com/yuyanky/todo/internal/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
