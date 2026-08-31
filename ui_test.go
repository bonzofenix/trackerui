package ui_test

import (
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

func TestStylesheetShipsTheTokens(t *testing.T) {
	css, err := ui.CSS()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"--violet: #8b5cf6", "--bg: #0b0b12", "--surface: #14141f",
		"--border: #2a2a3d", "--text: #f1f1f7", "--muted: #9a9ab5",
		"Barlow Condensed", "prefers-reduced-motion",
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
