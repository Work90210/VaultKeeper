package logging

import "testing"

func TestIsSensitiveField(t *testing.T) {
	cases := map[string]bool{
		"access_token": true,
		"jwt":          true,
		"bearer":       true,
		"password":     true,
		"client_secret": true,
		"api_key":       true,
		"full_name":     true,
		"contact_info":  true,
		"location":      true,
		"case_id":       false,
		"status":        false,
		"method":        false,
	}

	for field, expected := range cases {
		if actual := IsSensitiveField(field); actual != expected {
			t.Errorf("IsSensitiveField(%q) = %v, want %v", field, actual, expected)
		}
	}
}

func TestRedactMap(t *testing.T) {
	input := map[string]any{
		"case_reference": "CASE-2026",
		"api_key":        "secret-value",
		"nested": map[string]any{
			"contact_info": "private-data",
			"status":       "ok",
		},
		"tokens": []string{"one", "two"},
	}

	output := RedactMap(input)

	if output["case_reference"] != "CASE-2026" {
		t.Error("expected non-sensitive value to remain untouched")
	}
	if output["api_key"] != redactedValue {
		t.Error("expected api_key to be redacted")
	}

	nested, ok := output["nested"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map output")
	}
	if nested["contact_info"] != redactedValue {
		t.Error("expected nested contact_info to be redacted")
	}
	if nested["status"] != "ok" {
		t.Error("expected nested status to remain untouched")
	}
	if output["tokens"] != redactedValue {
		t.Error("expected slice under sensitive key to be redacted")
	}
}

func TestRedactMap_NilInput(t *testing.T) {
	if result := RedactMap(nil); result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}
