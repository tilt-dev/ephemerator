package static

import "embed"

// Content holds static web server files.
//go:embed index.html favicon.ico
var Content embed.FS
