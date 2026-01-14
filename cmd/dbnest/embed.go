package main

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedFrontend embed.FS

// frontendContent is the embedded filesystem for the frontend
var frontendContent fs.FS = embeddedFrontend
