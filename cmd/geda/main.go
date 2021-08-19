package main

import (
	"log"
	"kubeease.com/kubeease/geda/cmd/geda/app"
)

func main() {
	if err := app.NewCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
