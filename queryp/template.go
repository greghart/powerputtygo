package queryp

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Templater describes shared interface of Template and TemplateBuilder.
type Templater interface {
	Text() *template.Template
	Placeholderer(p Placeholderer) Templater
	Param(key string, val any) Templater
	Params(params map[string]any) Templater
	Include(associations ...string) Templater
	Execute() (string, []any, error)
}

// Template represents a SQL template.
// Any method on Template spins off a mutable builder so this can be re-used freely.
type Template struct {
	text *template.Template
}

func NewTemplate(text string) (*Template, error) {
	// For now, named templates aren't needed at all
	t, err := template.New("template").Parse(text)
	if err != nil {
		return nil, err
	}
	return &Template{
		text: t,
	}, nil
}

func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}
	return t
}

// Text returns the underlying text/template.Template.
func (t *Template) Text() *template.Template {
	return t.text
}

// Build returns a forked Templater that can be used to build custom data for the template.
func (t *Template) Build() Templater {
	return newTemplateBuilder(t)
}

// Placeholderer sets how to replaced named parameters (defaults to Sqlite style '?')
// Proxies to templateBuilder under the hood.
func (t *Template) Placeholderer(p Placeholderer) Templater {
	return t.Build().Placeholderer(p)
}

// Param sets a named parameter value.
// Proxies to templateBuilder under the hood.
func (t *Template) Param(key string, val any) Templater {
	return t.Build().Param(key, val)
}

// Params sets multiple named parameters at a time (additive with existing ones).
// Proxies to templateBuilder under the hood.
func (t *Template) Params(params map[string]any) Templater {
	return t.Build().Params(params)
}

// Include marks associations to be included in the template.
// Nested associations work with a DSV (dot-separated values) syntax, eg. `a.b.c`.
// Inspired by {https://jsonapi.org/format/#fetching-includes}
// Intermediate relationships must also be included.
// Eg. 'a.b.c' results in includes of a->b->c, and following queries would be true:
// `a.b.c`, `a.b`, , `a`, but NOT `b.c` or `c`.
// Proxies to templateBuilder under the hood.
func (t *Template) Include(associations ...string) Templater {
	return t.Build().Include(associations...)
}

// Execute executes the template and returns it as a string.
// Proxies to templateBuilder under the hood.
func (t *Template) Execute() (string, []any, error) {
	return t.Build().Execute()
}

////////////////////////////////////////////////////////////////////////////////

type TemplateBuilder struct {
	*Template
	params        map[string]any  // Store parameters
	includes      map[string]bool // Store included associations
	placeholderer Placeholderer
}

func newTemplateBuilder(t *Template) *TemplateBuilder {
	return &TemplateBuilder{
		Template: t,
		params:   make(map[string]any),
		includes: make(map[string]bool),
	}
}

func (t *TemplateBuilder) Text() *template.Template {
	return t.Template.Text()
}

func (t *TemplateBuilder) Placeholderer(p Placeholderer) Templater {
	t.placeholderer = p
	return t
}

func (t *TemplateBuilder) Param(key string, val any) Templater {
	return t.Params(map[string]any{key: val})
}

func (t *TemplateBuilder) Params(params map[string]any) Templater {
	for k, v := range params {
		t.params[k] = v
	}
	return t
}

func (t *TemplateBuilder) Include(associations ...string) Templater {
	for _, assoc := range associations {
		components := strings.Split(assoc, ".")
		for i := range len(components) {
			t.includes[strings.Join(components[:i+1], ".")] = true
		}
	}
	return t
}

func (t *TemplateBuilder) Execute() (string, []any, error) {
	buffer := &bytes.Buffer{}
	err := t.Template.text.Execute(buffer, t.data())
	if err != nil {
		return "", nil, err
	}
	// We also support NamedQuery style, which can be applied post template execution
	q, args := Named(buffer.String()).
		WithPlaceholderer(t.placeholderer).
		Params(t.params).
		Execute()
	return q, args, nil
}

func (t *TemplateBuilder) data() *templateData {
	return &templateData{
		params:   t.params,
		includes: t.includes,
	}
}

////////////////////////////////////////////////////////////////////////////////

// templateData is the data object a template will be executed against.
type templateData struct {
	params   map[string]any
	includes map[string]bool
}

func (t *templateData) Param(key string) string {
	if _, ok := t.params[key]; ok {
		return fmt.Sprintf(":%s", key)
	}
	return ""
}

func (t *templateData) Params() map[string]any {
	return t.params
}

func (t *templateData) HasParams() bool {
	return len(t.params) > 0
}

func (t *templateData) Includes(keys ...string) bool {
	for _, key := range keys {
		_, ok := t.includes[key]
		if ok {
			return true
		}
	}
	return false
}

func (t *templateData) Include(keys ...string) bool {
	return t.Includes(keys...)
}
