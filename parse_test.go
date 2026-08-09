package main

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
		wantNm  string
	}{
		{
			name:   "clean object",
			in:     `{"name":"Тень","abilities":{"str":8}}`,
			wantNm: "Тень",
		},
		{
			name:   "fenced with prose",
			in:     "Вот персонаж:\n```json\n{\"name\":\"Мерген\",\"abilities\":{\"dex\":16}}\n```\nготово",
			wantNm: "Мерген",
		},
		{
			name:    "no json",
			in:      "тут вообще нет объекта",
			wantErr: true,
		},
		{
			name:    "missing name",
			in:      `{"abilities":{"str":10}}`,
			wantErr: true,
		},
		{
			name:    "missing abilities",
			in:      `{"name":"Безрукий"}`,
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obj, err := extractJSON(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", obj)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if obj["name"] != c.wantNm {
				t.Fatalf("name = %v, want %q", obj["name"], c.wantNm)
			}
		})
	}
}

func TestNormalizeFillsDefaults(t *testing.T) {
	obj := map[string]any{
		"name":      "Тень",
		"abilities": map[string]any{"str": 8},
		"hp":        map[string]any{"max": 24.0},
	}
	normalize(obj)

	if obj["schema"] != "gm-character/v1" {
		t.Fatalf("schema = %v, want gm-character/v1", obj["schema"])
	}
	hp := obj["hp"].(map[string]any)
	if hp["current"] != 24.0 {
		t.Fatalf("hp.current = %v, want it defaulted to max 24", hp["current"])
	}
	if hp["temp"] != 0 {
		t.Fatalf("hp.temp = %v, want 0", hp["temp"])
	}
	if _, ok := obj["conditions"].([]any); !ok {
		t.Fatalf("conditions = %v, want empty slice", obj["conditions"])
	}
}

func TestNormalizeKeepsExplicitHP(t *testing.T) {
	obj := map[string]any{
		"name":      "Раненый",
		"abilities": map[string]any{},
		"hp":        map[string]any{"max": 30.0, "current": 7.0, "temp": 5.0},
	}
	normalize(obj)
	hp := obj["hp"].(map[string]any)
	if hp["current"] != 7.0 || hp["temp"] != 5.0 {
		t.Fatalf("normalize clobbered explicit hp: %#v", hp)
	}
}

func TestParseModelDefault(t *testing.T) {
	t.Setenv("PARSE_MODEL", "")
	if got := parseModel(); string(got) != "claude-opus-4-8" {
		t.Fatalf("default model = %q, want claude-opus-4-8", got)
	}
	t.Setenv("PARSE_MODEL", "claude-sonnet-5")
	if got := parseModel(); string(got) != "claude-sonnet-5" {
		t.Fatalf("override model = %q, want claude-sonnet-5", got)
	}
}
