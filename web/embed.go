package web

import "embed"

// Assets, gömülü statik web dosyalarını (index.html, style.css, app.js, global.css) içerir.
//
//go:embed index.html style.css app.js global.css
var Assets embed.FS
