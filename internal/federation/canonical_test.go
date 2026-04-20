package federation

import (
	"testing"
)

func TestCanonicalJSON_SortsKeys(t *testing.T) {
	// Map with unsorted keys must produce sorted JSON.
	input := map[string]any{
		"z": 1,
		"a": 2,
		"m": 3,
	}
	got, err := CanonicalJSON(input)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	want := `{"a":2,"m":3,"z":1}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCanonicalJSON_NestedObjects(t *testing.T) {
	input := map[string]any{
		"b": map[string]any{
			"z": true,
			"a": false,
		},
		"a": 1,
	}
	got, err := CanonicalJSON(input)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	want := `{"a":1,"b":{"a":false,"z":true}}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCanonicalJSON_Deterministic(t *testing.T) {
	input := map[string]any{
		"foo": "bar",
		"baz": []any{1, 2, 3},
	}
	first, _ := CanonicalJSON(input)
	for i := 0; i < 100; i++ {
		got, _ := CanonicalJSON(input)
		if string(got) != string(first) {
			t.Fatalf("non-deterministic at iteration %d: %s != %s", i, got, first)
		}
	}
}

func TestCanonicalHash_DomainSeparation(t *testing.T) {
	data := map[string]string{"key": "value"}

	hash1, err := HexHash("DOMAIN_A", data)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := HexHash("DOMAIN_B", data)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("different domains must produce different hashes")
	}
}

func TestCanonicalHash_SameInputSameOutput(t *testing.T) {
	data := map[string]any{"x": 1, "y": "z"}
	h1, _ := HexHash("TEST", data)
	h2, _ := HexHash("TEST", data)
	if h1 != h2 {
		t.Errorf("same input produced different hashes: %s vs %s", h1, h2)
	}
}
