package main

import (
	"linkerd-easyauth/cmd"
	"os"
)

func main() {
	if err := cmd.NewEasyAuthCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
