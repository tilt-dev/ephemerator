package template

import "embed"

// Content holds HTML template files.
//go:embed *.tmpl
var Content embed.FS
