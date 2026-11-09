package main

import (
	"os"

	"example.com/cmdcase/internal/compose/app"
)

func main() {
	os.Exit(app.Main(os.Args[1:]))
}
