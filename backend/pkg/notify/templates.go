package notify

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

// kv is a key/value row used by the reusable {{template "key_value_table"}}.
type kv struct{ K, V string }

// hero passes a headline + supporting line into the layout's hero block.
type hero struct{ Title, Subtitle string }

// cta is the primary action button.
type cta struct{ URL, Label string }

// amount renders the gradient summary box used by orders + refunds.
type amount struct{ Label, Amount, Sub string }

// alert is the colored callout box (used for success and warning variants).
type alert struct{ Title, Body string }

// otpCode renders the OTP digit grid + expiry hint.
type otpCode struct {
	Code    string
	Minutes int
}

// baseData is what every template's layout needs. Per-template structs
// embed this so callers don't repeat themselves.
type baseData struct {
	Subject   string
	HeaderTag string
	Year      int
}

func newBase(subject, headerTag string) baseData {
	return baseData{Subject: subject, HeaderTag: headerTag, Year: time.Now().UTC().Year()}
}

// Renderer holds parsed templates. One layout parsed per body template via
// a fresh clone to avoid the global {{define "body"}} collision across files.
type Renderer struct {
	templates map[string]*template.Template
}

func NewRenderer() (*Renderer, error) {
	// Load layout + partials once as a "base" tree.
	base, err := template.New("layout").ParseFS(templateFS,
		"templates/_layout.html", "templates/_partials.html")
	if err != nil {
		return nil, fmt.Errorf("parse layout: %w", err)
	}

	bodyFiles := []string{
		"order_placed.html",
		"order_status.html",
		"order_cancelled.html",
		"refund.html",
		"return.html",
		"otp.html",
		"low_stock.html",
	}

	r := &Renderer{templates: make(map[string]*template.Template, len(bodyFiles))}
	for _, f := range bodyFiles {
		clone, err := base.Clone()
		if err != nil {
			return nil, err
		}
		t, err := clone.ParseFS(templateFS, "templates/"+f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		// Use the file name (sans extension) as the lookup key.
		name := strings.TrimSuffix(f, ".html")
		r.templates[name] = t
	}
	return r, nil
}

// render executes the named template and returns the rendered HTML + a
// plain-text fallback derived by stripping tags.
func (r *Renderer) render(name string, data any) (htmlBody, textBody string, err error) {
	t, ok := r.templates[name]
	if !ok {
		return "", "", fmt.Errorf("unknown template: %s", name)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		return "", "", err
	}
	htmlBody = buf.String()
	textBody = htmlToText(htmlBody)
	return htmlBody, textBody, nil
}

// htmlToText is a tiny tag stripper for the text/plain fallback. Good
// enough for screen readers + spam filters; not a full Markdown render.
func htmlToText(s string) string {
	// Strip script/style blocks entirely.
	for _, tag := range []string{"script", "style"} {
		open := "<" + tag
		for {
			i := strings.Index(s, open)
			if i < 0 {
				break
			}
			j := strings.Index(s[i:], "</"+tag+">")
			if j < 0 {
				break
			}
			s = s[:i] + s[i+j+len(tag)+3:]
		}
	}
	// Drop all tags.
	var out strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out.WriteRune(r)
		}
	}
	// Collapse whitespace.
	lines := strings.Split(out.String(), "\n")
	clean := make([]string, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			clean = append(clean, ln)
		}
	}
	return strings.Join(clean, "\n")
}
