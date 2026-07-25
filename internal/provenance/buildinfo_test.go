package provenance

import "testing"

func TestFields(t *testing.T) {
	fields := Fields()
	if len(fields) != 3 {
		t.Fatalf("len(fields) = %d", len(fields))
	}
}
