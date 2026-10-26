package ui_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	ui "github.com/bonzofenix/trackerui"
)

type pageData struct {
	Layout ui.Layout
	Items  []string
}

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"templates/home.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<h1>Home</h1>{{range .Items}}<p>{{.}}</p>{{end}}{{end}}`)},
		"templates/other.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<h1>Other</h1>{{end}}` +
				`{{define "slot"}}<div class="slot">extra</div>{{end}}`)},
	}
}

func render(t *testing.T, page string, d pageData) string {
	t.Helper()
	r, err := ui.NewRenderer(testFS(), "templates", nil)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	var sb strings.Builder
	if err := r.Render(&sb, page, d); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return sb.String()
}

func TestEachPageRendersItsOwnContent(t *testing.T) {
	// Regression: parsing every page into one template set lets the last one
	// win, because they all define a block named "content".
	home := render(t, "home", pageData{Items: []string{"a"}})
	other := render(t, "other", pageData{})
	if !strings.Contains(home, "<h1>Home</h1>") {
		t.Error("home page did not render its own content")
	}
	if strings.Contains(home, "<h1>Other</h1>") {
		t.Error("home page leaked the other page's content")
	}
	if !strings.Contains(other, "<h1>Other</h1>") {
		t.Error("other page did not render its own content")
	}
}

func TestLayoutRendersBrandAndNav(t *testing.T) {
	out := render(t, "home", pageData{Layout: ui.Layout{
		Title: "Library",
		Brand: ui.Brand{Prefix: "TANGO", Suffix: "TRACKER", Href: "/"},
		Nav: []ui.NavItem{
			{Label: "Library", Href: "/", Current: true},
			{Label: "Tags", Href: "/tags"},
		},
	}})

	if !strings.Contains(out, `TANGO<span class="accent">TRACKER</span>`) {
		t.Error("split-accent wordmark missing")
	}
	if !strings.Contains(out, "<title>Library · TANGOTRACKER</title>") {
		t.Errorf("title wrong:\n%s", out[:min(400, len(out))])
	}
	if !strings.Contains(out, `href="/tags"`) {
		t.Error("nav item missing")
	}
	if !strings.Contains(out, `aria-current="page"`) {
		t.Error("current nav item not marked")
	}
}

func TestSlotIsOptional(t *testing.T) {
	// A page without a "slot" block must still render; the layout's default
	// empty block covers it.
	if out := render(t, "home", pageData{}); strings.Contains(out, `class="slot"`) {
		t.Error("home rendered a slot it never defined")
	}
	if out := render(t, "other", pageData{}); !strings.Contains(out, `class="slot"`) {
		t.Error("other page's slot was not rendered")
	}
}

func TestWidthOverridesContainerOnlyWhenSet(t *testing.T) {
	wide := render(t, "home", pageData{Layout: ui.Layout{Width: "1400px"}})
	if !strings.Contains(wide, "--container:1400px") {
		t.Error("Width did not override --container")
	}
	if narrow := render(t, "home", pageData{}); strings.Contains(narrow, "--container:") {
		t.Error("empty Width should leave the default container alone")
	}
}

func TestScriptsAreDeferred(t *testing.T) {
	out := render(t, "home", pageData{Layout: ui.Layout{
		Scripts: []string{"/static/app.js"}}})
	if !strings.Contains(out, `<script src="/static/app.js" defer></script>`) {
		t.Error("script tag missing or not deferred")
	}
}

func TestFontAndStylesheetAlwaysLinked(t *testing.T) {
	// An app that forgets the font link silently falls back to a system stack
	// and stops looking like the family, so the layout owns both links.
	out := render(t, "home", pageData{})
	if !strings.Contains(out, "fonts.googleapis.com/css2?family=Barlow+Condensed") {
		t.Error("Barlow font link missing")
	}
	if !strings.Contains(out, `href="/static/ui.css?v=`) {
		t.Error("stylesheet link missing or unversioned")
	}
}

func TestUnknownPageIsAnError(t *testing.T) {
	r, err := ui.NewRenderer(testFS(), "templates", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Render(&strings.Builder{}, "nope", pageData{}); err == nil {
		t.Error("want an error for an unknown page")
	}
}

func TestEmptyTemplateDirIsAnError(t *testing.T) {
	empty := fstest.MapFS{"templates/readme.txt": &fstest.MapFile{Data: []byte("x")}}
	if _, err := ui.NewRenderer(empty, "templates", nil); err == nil {
		t.Error("want an error when no page templates exist")
	}
}

// The skip link is the first focusable thing on the page, and it has to come
// before the top bar in DOM order to be reachable first. Ordering is the whole
// feature: a skip link rendered after the nav it skips is worse than none,
// because it looks like the problem is solved.
func TestSkipLinkComesBeforeTheTopBar(t *testing.T) {
	out := render(t, "home", pageData{Layout: ui.Layout{
		Nav: []ui.NavItem{{Label: "Tags", Href: "/tags"}},
	}})

	skip := strings.Index(out, `<a class="skip-link" href="#main">`)
	if skip < 0 {
		t.Fatal("no skip link")
	}
	bar := strings.Index(out, `<header class="topbar">`)
	if bar < 0 {
		t.Fatal("no top bar")
	}
	if skip > bar {
		t.Error("the skip link renders after the bar it is meant to skip")
	}

	// It has to land somewhere. An href with no target scrolls nowhere and
	// silently does nothing.
	if !strings.Contains(out, `<main id="main"`) {
		t.Error(`skip link targets #main, but <main> carries no id="main"`)
	}
	// Without tabindex the target takes the reading position but not focus, so
	// the next Tab starts again from the top of the document.
	if !strings.Contains(out, `tabindex="-1"`) {
		t.Error("<main> is not focusable, so the skip link moves reading position only")
	}

	// The two halves have to agree. Asserted separately above, renaming one
	// leaves a link that scrolls nowhere and still passes.
	href := out[skip+len(`<a class="skip-link" href="#`):]
	href = href[:strings.Index(href, `"`)]
	if !strings.Contains(out, `<main id="`+href+`"`) {
		t.Errorf("skip link points at #%s, which no element carries as its id", href)
	}
}

// The layout must contribute exactly one id="main". A consuming app that
// declares its own would give the document two, and the fragment would resolve
// to whichever came first -- silently landing the reader inside the content
// rather than at the top of it. This module cannot prevent that, so what is
// pinned here is that the layout itself contributes one and only one; the
// duplicate case is a documented constraint on consumers.
func TestLayoutContributesExactlyOneSkipTarget(t *testing.T) {
	out := render(t, "home", pageData{})
	if n := strings.Count(out, `id="main"`); n != 1 {
		t.Errorf(`layout renders id="main" %d times, want exactly 1`, n)
	}
}

// A fixed dark palette that does not declare itself gets light scrollbars and
// light <select> drop-downs drawn over it by the browser.
func TestLayoutDeclaresTheDarkColorScheme(t *testing.T) {
	out := render(t, "home", pageData{})
	if !strings.Contains(out, `<meta name="color-scheme" content="dark">`) {
		t.Error("no color-scheme declaration")
	}
	if !strings.Contains(out, `<meta name="theme-color" content="#0b0b12">`) {
		t.Error("no theme-color, so mobile browser chrome will not match --bg")
	}
	// The meta and the manifest render ThemeColor, which is a hand-kept copy
	// of --bg. Pin the copy to the stylesheet itself, not to a literal.
	css, err := ui.CSS()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "--bg: "+ui.ThemeColor) {
		t.Errorf("ThemeColor %s is not --bg in the stylesheet", ui.ThemeColor)
	}
}

// Without these three links iOS puts a page screenshot on the home screen,
// Chrome cannot install the app, and tabs fall back to /favicon.ico.
func TestIconLinksRenderOnlyWhenIconSet(t *testing.T) {
	out := render(t, "home", pageData{Layout: ui.Layout{Icon: "/static/icon.png"}})
	for _, want := range []string{
		`<link rel="icon" type="image/png" href="/static/icon.png">`,
		`<link rel="apple-touch-icon" href="/static/icon.png">`,
		`<link rel="manifest" href="` + ui.ManifestPath + `">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}

	// An app that has not opted in must not link a manifest it does not
	// serve, or every page load logs a 404.
	bare := render(t, "home", pageData{})
	for _, rel := range []string{`rel="icon"`, `rel="apple-touch-icon"`, `rel="manifest"`} {
		if strings.Contains(bare, rel) {
			t.Errorf("empty Icon still rendered %s", rel)
		}
	}
}

func TestManifestDescribesTheApp(t *testing.T) {
	h := ui.ManifestHandler(ui.Brand{Prefix: "TANGO", Suffix: "TRACKER", Href: "/"}, "/static/icon.png")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ui.ManifestPath, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("Content-Type = %q", ct)
	}

	// Decode into maps, not tagged structs: encoding/json matches keys
	// case-insensitively, but browsers do not, so "Src" would pass a struct.
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}
	// The name matches the <title> wordmark, so the home-screen label and
	// the tab read the same.
	for key, want := range map[string]string{
		"name":             "TANGOTRACKER",
		"short_name":       "TANGOTRACKER",
		"start_url":        "/",
		"display":          "standalone",
		"theme_color":      ui.ThemeColor,
		"background_color": ui.ThemeColor,
	} {
		if got := m[key]; got != want {
			t.Errorf("%s = %v, want %q", key, got, want)
		}
	}

	icons, _ := m["icons"].([]any)
	if len(icons) != 1 {
		t.Fatalf("icons = %v, want one", m["icons"])
	}
	icon, _ := icons[0].(map[string]any)
	for key, want := range map[string]string{
		"src": "/static/icon.png", "sizes": "512x512", "type": "image/png",
	} {
		if got := icon[key]; got != want {
			t.Errorf("icons[0].%s = %v, want %q", key, got, want)
		}
	}
}

// Brand.Href is optional in the layout (the wordmark falls back to "/"), so
// the manifest must not emit an empty start_url.
func TestManifestStartURLDefaultsToRoot(t *testing.T) {
	body := ui.Manifest(ui.Brand{Prefix: "PHOTO", Suffix: "TRACKER"}, "/i.png")
	if !strings.Contains(string(body), `"start_url":"/"`) {
		t.Errorf("start_url not defaulted: %s", body)
	}
}

func TestStylesheetShipsTheTokens(t *testing.T) {
	css, err := ui.CSS()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"--violet: #8b5cf6", "--bg: #0b0b12", "--surface: #14141f",
		"--border: #2a2a3d", "--text: #f1f1f7", "--muted: #9a9ab5",
		"Barlow Condensed", "prefers-reduced-motion",
		// Both the <meta> and the property are kept. They set the same
		// value, so this is not belt-and-braces: the <meta> applies before
		// the stylesheet has loaded, which is what stops a flash of light
		// browser chrome on a slow connection.
		"color-scheme: dark",
		// Without this the sticky top bar covers whatever the skip link
		// jumped to, which looks exactly like the link doing nothing.
		"scroll-padding-top",
		// Presence guards, not behaviour: a stylesheet cannot be rendered
		// here, so these only catch a rule deleted by accident.
		".skip-link",
		"button:focus-visible",
		"touch-action: manipulation",
	} {
		if !strings.Contains(string(css), want) {
			t.Errorf("stylesheet is missing %q", want)
		}
	}
}

func TestHumanSize(t *testing.T) {
	for _, c := range []struct {
		in   int64
		want string
	}{
		{512, "512 B"}, {10_000_000, "10.0 MB"}, {8_100_000_000, "8.1 GB"},
	} {
		if got := ui.HumanSize(c.in); got != c.want {
			t.Errorf("HumanSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRatioHandlesZeroTotal(t *testing.T) {
	if got := ui.Ratio(0, 0); got != "0%" {
		t.Errorf("Ratio(0,0) = %q, want 0%%", got)
	}
	if got := ui.Ratio(192, 334); got != "57%" {
		t.Errorf("Ratio(192,334) = %q, want 57%%", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestStylesLoadAfterTheSharedSheet(t *testing.T) {
	// An app's own CSS must come after trackerui's so it can override, and it
	// must be a <link>, not a <script> -- passing a stylesheet in Scripts is
	// an easy mistake that silently drops the styles.
	out := render(t, "home", pageData{Layout: ui.Layout{
		Styles: []string{"/static/app.css"}}})
	shared := strings.Index(out, "/static/ui.css")
	own := strings.Index(out, `<link rel="stylesheet" href="/static/app.css">`)
	if own < 0 {
		t.Fatal("app stylesheet not linked")
	}
	if shared > own {
		t.Error("app stylesheet must load after the shared one")
	}
}

// ---- Stylesheet accessibility guards -------------------------------------

func cssText(t *testing.T) string {
	t.Helper()
	b, err := ui.CSS()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func token(t *testing.T, css, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(name) + `:\s*(#[0-9a-fA-F]{6})`).FindStringSubmatch(css)
	if m == nil {
		t.Fatalf("token %s not found as a hex value", name)
	}
	return m[1]
}

func luminance(t *testing.T, hex string) float64 {
	t.Helper()
	var l float64
	for i, w := range []float64{0.2126, 0.7152, 0.0722} {
		v, err := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		if err != nil {
			t.Fatal(err)
		}
		c := float64(v) / 255
		if c <= 0.03928 {
			c /= 12.92
		} else {
			c = math.Pow((c+0.055)/1.055, 2.4)
		}
		l += w * c
	}
	return l
}

func contrast(t *testing.T, a, b string) float64 {
	la, lb := luminance(t, a), luminance(t, b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestTextTokensMeetAA(t *testing.T) {
	css := cssText(t)
	faint := token(t, css, "--faint")
	for _, bg := range []string{"--bg", "--surface", "--surface-2"} {
		if r := contrast(t, faint, token(t, css, bg)); r < 4.5 {
			t.Errorf("--faint on %s = %.2f:1, want >= 4.5", bg, r)
		}
	}
	m := regexp.MustCompile(`(?s)\n\.btn \{.*?\n\}`).FindString(css)
	bm := regexp.MustCompile(`background:\s*var\((--[a-z-]+)\)`).FindStringSubmatch(m)
	if bm == nil {
		t.Fatal(".btn background is not a token")
	}
	if r := contrast(t, "#ffffff", token(t, css, bm[1])); r < 4.5 {
		t.Errorf("white on .btn (%s) = %.2f:1, want >= 4.5", bm[1], r)
	}
	if r := contrast(t, "#ffffff", token(t, css, "--violet-darker")); r < 4.5 {
		t.Errorf("white on .btn:hover = %.2f:1, want >= 4.5", r)
	}
}

// block returns the body of the first top-level @media or rule whose header
// contains head.
func block(t *testing.T, css, head string) string {
	t.Helper()
	i := strings.Index(css, head)
	if i < 0 {
		t.Fatalf("no %q block", head)
	}
	depth, start := 0, strings.Index(css[i:], "{")+i
	for j := start; j < len(css); j++ {
		switch css[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return css[start+1 : j]
			}
		}
	}
	t.Fatalf("unterminated %q block", head)
	return ""
}

func TestCoarsePointerTargetsAre44px(t *testing.T) {
	css := cssText(t)
	coarse := block(t, css, "@media (pointer: coarse)")
	for _, sel := range []string{".btn-sm", ".btn-del", "a.tag", ".chip", ".topbar nav a"} {
		re := regexp.MustCompile(`(?m)(^|[\s,])` + regexp.QuoteMeta(sel) + `\s*(,[^{]*)?\{[^}]*min-height:\s*44px`)
		if !re.MatchString(coarse) {
			t.Errorf("%s has no min-height:44px under (pointer: coarse)", sel)
		}
	}
	if !strings.Contains(css, "padding: var(--s2); min-height: 44px") {
		t.Error("narrow-screen nav links are not 44px")
	}
	// The coarse block must come last so it outranks the base rules.
	if strings.Index(css, "@media (pointer: coarse)") < strings.LastIndex(css, ".btn-sm {") {
		t.Error("coarse-pointer block precedes .btn-sm base rule and would lose")
	}
}

func TestScriptFocusedElementsGetARing(t *testing.T) {
	css := cssText(t)
	re := regexp.MustCompile(`(?s)\[tabindex="-1"\]:focus-visible\s*\{[^}]*outline:\s*2px solid`)
	if !re.MatchString(css) {
		t.Error(`[tabindex="-1"]:focus-visible has no outline`)
	}
}

func TestChipComponent(t *testing.T) {
	css := cssText(t)
	for _, want := range []string{".chip {", ".chip-count", `.chip[aria-current="page"]`,
		`.chip[aria-pressed="true"]`, "text-overflow: ellipsis", "white-space: nowrap",
		`input[type="checkbox"]`, "::placeholder"} {
		if !strings.Contains(css, want) {
			t.Errorf("stylesheet is missing %q", want)
		}
	}
}
