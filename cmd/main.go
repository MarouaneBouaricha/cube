package main

import (
	"github.com/MarouaneBouaricha/cube/cmd/cli"
	"log"
)

func main() {
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
}
