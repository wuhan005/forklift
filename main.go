package main

import (
	"os"
	"os/signal"
	"syscall"

	_ "github.com/samber/lo"
)

func main() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}
