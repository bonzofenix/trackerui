package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path"
	"strings"
)

//go:embed templates/layout.html
var layoutFS embed.FS

// Renderer holds one parsed template per page.
//
// Each page is parsed into its own template set rather than one shared set,
// because every page defines a block named "content": in a single set the
// last file parsed would win and every page would render identically.
type Renderer struct {
	pages map[string]*template.Template
}

// NewRenderer parses every *.html under dir in fsys as a page, combining each
// with the shared layout.
//
// Page templates must define "content" and may define "slot" to fill the
// top-bar's right-hand area. Funcs are merged over the built-ins, so an app
// can override a helper it needs to behave differently.
func NewRenderer(fsys fs.FS, dir string, funcs template.FuncMap) (*Renderer, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	merged := template.FuncMap{}
	for k, v := range builtins {
		merged[k] = v
	}
	for k, v := range funcs {
		merged[k] = v
	}

	r := &Renderer{pages: map[string]*template.Template{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".html")
		t, err := template.New(name).Funcs(merged).
			ParseFS(layoutFS, "templates/layout.html")
		if err != nil {
			return nil, fmt.Errorf("parse layout: %w", err)
		}
		if _, err := t.ParseFS(fsys, path.Join(dir, e.Name())); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		r.pages[name] = t
	}
	if len(r.pages) == 0 {
		return nil, fmt.Errorf("no page templates found in %s", dir)
	}
	return r, nil
}

// Render writes one page. Data must expose a Layout field of type Layout.
func (r *Renderer) Render(w io.Writer, page string, data any) error {
	t, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("unknown page %q", page)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

// Pages lists the parsed page names, for tests and diagnostics.
func (r *Renderer) Pages() []string {
	out := make([]string, 0, len(r.pages))
	for k := range r.pages {
		out = append(out, k)
	}
	return out
}

var builtins = template.FuncMap{
	"fontLink":   func() template.HTML { return template.HTML(FontLink) },
	"stylesheet": StylesheetHref,
	"humanSize":  HumanSize,
	"ratio":      Ratio,
}

// HumanSize formats a byte count in SI units, as file sizes are quoted on
// disk and in storage bills.
func HumanSize(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// Ratio renders part/total as a percentage string. Named "ratio" rather than
// "pct" deliberately: gymtracker's pct is a signed delta and tangotracker's
// was a proportion, so the shared name would have meant two things.
func Ratio(part, total int) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(part)/float64(total))
}
