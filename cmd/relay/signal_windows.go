package main

import (
	"os"
	"os/signal"
)

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	signal.Stop(ch)
}