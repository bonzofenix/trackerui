// Package ui is the shared look and feel for the *tracker apps
// (gymtracker, foodtracker, tangotracker).
//
// It exists because the design was previously shared by copy-paste:
// foodtracker's stylesheet carried a comment saying its palette "follow[s]
// gymtracker's app.css", which is exactly the arrangement that drifts. The
// stylesheet here is the single copy.
//
// The theme is fixed on purpose: one violet-on-cool-black dark theme, no
// light mode, no toggle, no per-app accent. Apps differ in their content and
// their container width, not their palette -- that is what makes them read as
// one family. Only --container is meant to be overridden, via Layout.Width.
//
// Usage:
//
//	//go:embed templates/*.html
//	var tmplFS embed.FS
//
//	r, err := ui.NewRenderer(tmplFS, "templates", nil)
//	mux.Handle("GET /static/", ui.StaticHandler())
//	r.Render(w, "library", data)
//
// Page templates define a "content" block and receive whatever data the app
// passes; the layout is supplied by this package.
package ui

import (
	"embed"
	"net/http"
	"time"
)

//go:embed static/ui.css
var staticFS embed.FS

// FontLink is the Google Fonts stylesheet the design depends on. Both existing
// apps load this exact triplet; it is here so a new app cannot forget it and
// silently fall back to a system stack.
const FontLink = `<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Barlow+Condensed:wght@500;600;700&family=Barlow:wght@400;500;600;700&display=swap">`

// NavItem is one link in the top bar.
type NavItem struct {
	Label string
	Href  string
	// Current marks the active page so the bar can highlight it.
	Current bool
}

// Brand is the split-accent wordmark: Prefix in plain text, Suffix in the
// accent colour. Every app in the family is "<something>" + "TRACKER".
type Brand struct {
	Prefix string // "TANGO"
	Suffix string // "TRACKER"
	Href   string // where the wordmark links, usually "/"
}

// Layout is the per-page shell data. Apps embed this in their own page struct
// so templates can reach both at once.
type Layout struct {
	Title string
	Brand Brand
	Nav   []NavItem
	// Width overrides --container (e.g. "1400px" for a media grid). Empty
	// keeps the 960px default.
	Width string
	// Scripts are extra <script src> URLs. Each app embeds and serves its own
	// JS: the shared package deliberately does not vendor htmx or Chart.js,
	// so an app that needs neither does not ship them.
	Scripts []string
}

// StaticHandler serves the shared stylesheet at /static/ui.css.
//
// The CSS is embedded rather than read from disk so a binary is
// self-contained; gymtracker learned this the hard way when a missing asset
// silently broke the page.
func StaticHandler() http.Handler {
	return http.StripPrefix("/", http.FileServer(http.FS(staticFS)))
}

// CSS returns the raw stylesheet, for apps that would rather inline it into
// their layout than serve a second request.
func CSS() ([]byte, error) { return staticFS.ReadFile("static/ui.css") }

// buildTime stamps the asset URL so a deploy busts the browser cache without
// anyone having to remember to bump a version.
var buildTime = time.Now().Unix()

// StylesheetHref is the URL to link in <head>.
func StylesheetHref() string {
	return "/static/ui.css?v=" + itoa(buildTime)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
