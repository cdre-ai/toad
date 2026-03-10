package cmd

import _ "embed"

//go:embed web/dashboard.html
var dashboardHTML string

//go:embed web/kiosk.html
var kioskHTML string
