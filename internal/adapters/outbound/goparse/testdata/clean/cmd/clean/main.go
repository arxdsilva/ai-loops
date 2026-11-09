// Command clean is wiring only.
package main

import (
	"os"

	"example.com/clean/internal/compose/app"
)

func main() {
	os.Exit(app.Main(os.Args[1:]))
}
