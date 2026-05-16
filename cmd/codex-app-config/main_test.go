package main

import "testing"

func TestModelForApplyKeepsExplicitModel(t *testing.T) {
	model := modelForApply("gpt-5.4-mini", map[string]string{
		"CODEX_MODEL": "gpt-5.5",
	})
	if model != "gpt-5.4-mini" {
		t.Fatalf("model = %q", model)
	}
}

func TestModelForApplyUsesEnvOrDefault(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "env", env: map[string]string{"CODEX_MODEL": "gpt-5.4"}, want: "gpt-5.4"},
		{name: "default", env: map[string]string{}, want: "gpt-5.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := modelForApply("", tc.env); got != tc.want {
				t.Fatalf("model = %q, want %q", got, tc.want)
			}
		})
	}
}
