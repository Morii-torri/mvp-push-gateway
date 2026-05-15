package msgtemplate

import (
	"strings"

	"github.com/flosch/pongo2/v6"
)

type TemplateEngine interface {
	Compile(source string) error
	Render(source string, context map[string]any) (string, error)
}

type pongoTemplateEngine struct{}

const missingPayloadFallback = "-"

func DefaultTemplateEngine() TemplateEngine {
	return pongoTemplateEngine{}
}

func (pongoTemplateEngine) Compile(source string) error {
	_, err := pongo2.FromString(prepareTemplateSource(source))
	return err
}

func (pongoTemplateEngine) Render(source string, context map[string]any) (string, error) {
	tpl, err := pongo2.FromString(prepareTemplateSource(source))
	if err != nil {
		return "", err
	}
	if context == nil {
		context = map[string]any{}
	}
	return tpl.Execute(pongo2.Context(context))
}

func prepareTemplateSource(source string) string {
	return applyGlobalPayloadFallback(normalizeDefaultFilterSyntax(source))
}

func applyGlobalPayloadFallback(templateBody string) string {
	return payloadVariablePattern.ReplaceAllStringFunc(templateBody, func(match string) string {
		parts := payloadVariablePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		expression := strings.TrimSpace(parts[1])
		if expression == "" || !payloadPathPattern.MatchString(expression) || defaultFilterPattern.MatchString(expression) {
			return match
		}
		return `{{ ` + expression + ` | default:"` + missingPayloadFallback + `" }}`
	})
}
