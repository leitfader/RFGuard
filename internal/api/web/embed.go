package web

import "embed"

//go:embed index.html access.html alerts.html styles.css app.js access.js alerts.js
var FS embed.FS
