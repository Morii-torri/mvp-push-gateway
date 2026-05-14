package msgtemplate

import "github.com/flosch/pongo2/v6"

type TemplateEngine interface {
	Compile(source string) error
	Render(source string, context map[string]any) (string, error)
}

type pongoTemplateEngine struct{}

func DefaultTemplateEngine() TemplateEngine {
	return pongoTemplateEngine{}
}

func (pongoTemplateEngine) Compile(source string) error {
	_, err := pongo2.FromString(normalizeDefaultFilterSyntax(source))
	return err
}

func (pongoTemplateEngine) Render(source string, context map[string]any) (string, error) {
	tpl, err := pongo2.FromString(normalizeDefaultFilterSyntax(source))
	if err != nil {
		return "", err
	}
	if context == nil {
		context = map[string]any{}
	}
	return tpl.Execute(pongo2.Context(context))
}
