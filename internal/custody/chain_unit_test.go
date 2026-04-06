package custody

import (
	"testing"
)

// TestSortedJSON_MarshalError exercises the json.Marshal error branch in sortedJSON.
// json.Marshal fails on values that cannot be serialized, such as channels.
// We inject a channel into the map to trigger the error path that returns "{}".
func TestSortedJSON_MarshalError(t *testing.T) {
	// A channel value cannot be JSON-marshaled; this forces the err != nil branch.
	m := map[string]any{
		"key": make(chan int),
	}
	result := sortedJSON(m)
	if result != "{}" {
		t.Errorf("sortedJSON with un-marshalable value = %q, want %q", result, "{}")
	}
}
