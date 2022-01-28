package static

import "embed"

// Content holds static web server files.
//go:embed *
var Content embed.FS
