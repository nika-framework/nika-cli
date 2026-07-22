package internal

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMockLiteralStringSlice(t *testing.T) {
	raw := json.RawMessage(`["one","two"]`)
	literal, _, _, err := mockLiteral("[]string", "tags", raw)
	if err != nil {
		t.Fatalf("mockLiteral() error = %v", err)
	}
	if literal != `[]string{"one", "two"}` {
		t.Fatalf("literal = %q, want %q", literal, `[]string{"one", "two"}`)
	}
}

func TestValidateRoutePlan(t *testing.T) {
	plan := aiRouteSpec{
		Operation:  "mock_data",
		Module:     "news",
		RouteName:  "CreateMock",
		HTTPMethod: "post",
		Path:       "/newss/mock",
	}
	if err := validateRoutePlan(&plan, "news"); err != nil {
		t.Fatalf("validateRoutePlan() error = %v", err)
	}
	if plan.HTTPMethod != "POST" {
		t.Fatalf("HTTPMethod = %q, want POST", plan.HTTPMethod)
	}
}

func TestBuildMockRouteUsesHandlerSuffixAndSwagger(t *testing.T) {
	plan := aiRouteSpec{
		RouteName:  "CreateMock",
		HTTPMethod: "POST",
		Path:       "/newss/mock",
		Values: map[string]json.RawMessage{
			"title": json.RawMessage(`"Mock title"`),
		},
	}
	source, err := buildMockRoute(plan, "NewsController", "news", "type News struct {\n\tTitle string `bson:\"title\"`\n}")
	if err != nil {
		t.Fatalf("buildMockRoute() error = %v", err)
	}
	for _, want := range []string{
		"func (c *NewsController) CreateMockHandler(",
		"@Summary Create mock News",
		"@Router /newss/mock [post]",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated route does not contain %q", want)
		}
	}
}
