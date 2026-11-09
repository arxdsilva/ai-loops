package order

import (
	"net/http"
	"os"

	"github.com/x/y"

	"example.com/imports/internal/adapters/outbound/db"
	"example.com/imports/internal/compose/app"
)

var _, _, _, _, _ = http.MethodGet, os.Args, y.Z, db.DB{}, app.Main
