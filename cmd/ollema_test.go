package cmd

import "testing"

func TestIsModulePrompt(t *testing.T) {
	for _, test := range []struct {
		prompt string
		want   bool
	}{
		{prompt: "Explain dependency injection in Go", want: false},
		{prompt: "لطفن یک ماژول خبر بساز", want: true},
		{prompt: "create a news module", want: true},
	} {
		if got := isModulePrompt(test.prompt); got != test.want {
			t.Errorf("isModulePrompt(%q) = %v, want %v", test.prompt, got, test.want)
		}
	}
}

func TestIsRoutePrompt(t *testing.T) {
	for _, test := range []struct {
		prompt string
		want   bool
	}{
		{prompt: "لطفن روت دیتای ماک روی news بساز", want: true},
		{prompt: "Add a mock endpoint to news", want: true},
		{prompt: "Create a news module", want: false},
	} {
		if got := isRoutePrompt(test.prompt); got != test.want {
			t.Errorf("isRoutePrompt(%q) = %v, want %v", test.prompt, got, test.want)
		}
	}
}
