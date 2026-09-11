# trackerui

The shared look and feel for the `*tracker` apps — gymtracker, foodtracker,
tangotracker.

It exists because the design was previously shared by copy-paste. foodtracker's
stylesheet carried a comment saying its palette "follow[s] gymtracker's
app.css" — an arrangement that drifts the moment either side is touched. This
package is the single copy.

## The theme is fixed

One violet-on-cool-black dark theme. No light mode, no toggle, no per-app
accent. The apps differ in content, not palette — that is what makes them read
as one family.

| token | value | |
|---|---|---|
| `--violet` | `#8b5cf6` | accent |
| `--violet-bright` | `#a78bfa` | accent on dark, wordmark |
| `--violet-soft` | `rgba(139,92,246,.14)` | accent fill |
| `--bg` | `#0b0b12` | ground |
| `--surface` / `--surface-2` / `--surface-3` | `#14141f` / `#1c1c2b` / `#24243a` | elevation |
| `--border` | `#2a2a3d` | |
| `--text` / `--muted` / `--faint` | `#f1f1f7` / `#9a9ab5` / `#6c6c86` | text ramp |
| `--ok` / `--warn` / `--danger` | `#22c55e` / `#f59e0b` / `#ef4444` | status, never themed |

Type is Barlow Condensed (headings, 700, uppercase) over Barlow (body).

Only `--container` is meant to vary, via `Layout.Width` — a media grid wants
more room than a dashboard.

**Depth comes from border and surface elevation, not shadows.** The only
shadows in the design are two button hovers. Don't add a shadow scale.

## Use

```go
//go:embed templates/*.html
var tmplFS embed.FS

r, err := ui.NewRenderer(tmplFS, "templates", myFuncs)
mux.Handle("GET /static/", ui.StaticHandler())

r.Render(w, "library", data)   // data has a Layout field
```

Each page template defines `content`, and may define `slot` to fill the top
bar's right-hand area:

```html
{{define "content"}}<h1>Library</h1>{{end}}
{{define "slot"}}<div class="slot">192 organized</div>{{end}}
```

Pages are parsed one set each, because they all define `content` — a single
set would let the last file win and every page would render identically.

## Components

`.card` `.tile` `.block` (+`.ok`/`.warn`/`.danger`/`.idle`) `.grid` `.stat`
`.badge` (+`.accent`/`.ok`/`.warn`/`.danger`) `.tag` `.btn` `.btn-ghost`
`.btn-sm` `.btn-del` `.tablewrap` `.empty` `.eyebrow` `.row-between`
`.muted` `.small` `.nums`

Badges follow one formula: solid text colour over the same hue at ~13% alpha.

## What the layout reserves

Two names are load-bearing, and a page that reuses either breaks something
silently rather than visibly.

`id="main"` — the layout puts it on `<main>` as the skip link's target. A
content block that declares its own gives the document two, and the fragment
resolves to whichever comes first, dropping the reader inside the content
instead of at the top of it. Nothing reports this.

`.skip-link` — the first focusable element on every page.

The layout also sets `scroll-padding-top: 4.5rem` on `<html>`, so that jumps to
a fragment clear the sticky top bar. An app with a taller bar should override
it; one that removes the bar entirely can set it to `0`.

## Overriding

`Layout.Styles` loads after the shared sheet, so an app's own rules win **at
equal specificity**. They do not win automatically: a shared rule written as a
type selector (`button:focus-visible`, (0,2,1)) outranks an app's bare class
(`.heart:focus-visible`, (0,2,0)) regardless of load order. When overriding
something the shared sheet styles by element, match or beat its specificity.

## Releasing, and what a stale branch costs

Every app pins a version, so a change here reaches production only when each
app bumps its pin. Two things follow from that.

A branch cut before someone else's change landed will **revert it on merge,
with no conflict to warn anyone.** Git sees the older branch as deleting the
lines it never had, the diff shows them as ordinary removals, and GitHub
reports the PR mergeable. This is not hypothetical: #1 was branched before #2
and would have silently removed `min-width: 0` from `.topbar nav`, putting the
tabs back off the edge of the screen on every phone — a regression invisible at
desktop width. Reading the diff does not catch it. Checking the merge result
does:

```sh
# Read the rule out of the MERGED FILE, not out of a diff.
tree=$(git merge-tree --write-tree origin/main <branch> | head -1 | tr -d '\r\n')
blob=$(git ls-tree "$tree" static/ui.css | awk '{print $3}')
git cat-file -p "$blob" | grep -c 'min-width: 0'                 # 0 = gone
```

Grepping `git merge-tree`'s own output instead is this same mistake one level
down, and it sat in this section until someone ran it: that output is the
changed hunks, so a rule neither side touched never appears in it and reads as
missing when it is fine. It answers "did this hunk change", not "is the rule
still there". Only the merged file answers the second, and the second is the
question.

Two details the three lines above exist for. `--write-tree` prints a trailing
carriage return, which silently corrupts the SHA when it is interpolated into a
path. And `<tree-sha>:<path>` does not resolve against a bare tree the way it
does against a commit, so the blob is looked up with `ls-tree` rather than
addressed through the tree. Both fail by returning a clean `0` — the same
answer as a genuinely missing rule.

Rebase any branch older than the last merge here before merging it, and check
the result rather than the diff.

The same hazard arrives downstream one step later: bumping an app's pin to a
version built from a main that lost a rule ships that loss to production, where
it looks like the app's own regression. Bump from a main you have checked.

## JavaScript

None is vendored. gymtracker needs htmx, foodtracker needs Chart.js,
tangotracker needs neither — so each app embeds and serves its own and passes
URLs via `Layout.Scripts`. Vendor rather than CDN: gymtracker already had a
CDN miss silently break every interaction on the page.
