// Package web provides embedded static assets for the web user interface.
package web

import "embed"

// Assets embeds the compiled frontend production distribution.
//
//go:embed dist/*
var Assets embed.FS
