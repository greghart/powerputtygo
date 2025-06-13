package queryp

import (
	"bytes"
	"fmt"
	"text/template"
)

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

// Build returns a TemplateBuilder that can be used to build custom data for the template.
func (t *Template) Build() *TemplateBuilder {
	return newTemplateBuilder(t)
}

// Placeholderer sets how to replaced named parameters (defaults to Sqlite style '?')
// Proxies to templateBuilder under the hood.
func (t *Template) Placeholderer(p Placeholderer) *TemplateBuilder {
	return t.Build().Placeholderer(p)
}

// Param sets a named parameter value.
// Proxies to templateBuilder under the hood.
func (t *Template) Param(key string, val any) *TemplateBuilder {
	return t.Build().Param(key, val)
}

// Params sets multiple named parameters at a time (additive with existing ones).
// Proxies to templateBuilder under the hood.
func (t *Template) Params(params map[string]any) *TemplateBuilder {
	return t.Build().Params(params)
}

// Include marks associations to be included in the template.
// Proxies to templateBuilder under the hood.
func (t *Template) Include(associations ...string) *TemplateBuilder {
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

func (t *TemplateBuilder) Placeholderer(p Placeholderer) *TemplateBuilder {
	t.placeholderer = p
	return t
}

func (t *TemplateBuilder) Param(key string, val any) *TemplateBuilder {
	return t.Params(map[string]any{key: val})
}

func (t *TemplateBuilder) Params(params map[string]any) *TemplateBuilder {
	for k, v := range params {
		t.params[k] = v
	}
	return t
}

func (t *TemplateBuilder) Include(associations ...string) *TemplateBuilder {
	for _, assoc := range associations {
		t.includes[assoc] = true
	}
	return t
}

func (t *TemplateBuilder) Execute() (string, []any, error) {
	data := &templateData{
		params:   t.params,
		includes: t.includes,
	}
	buffer := &bytes.Buffer{}
	err := t.Template.text.Execute(buffer, data)
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
