package main

import (
	"log"
	"os"

	"github.com/hasirciogluhq/atellar/internal/agent"
)

func main() {
	if err := agent.Run(); err != nil {
		log.Printf("atellar agent stopped: %v", err)
		os.Exit(1)
	}
}
