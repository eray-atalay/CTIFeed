package web

import "embed"

// Assets, Astro tarafından web/dist klasörüne derlenen statik varlıkları içerir.
//
//go:embed dist/*
var Assets embed.FS
