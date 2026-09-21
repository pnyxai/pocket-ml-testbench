package records

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// Old buffers in production have no identity_state key at all.
func TestFallbackAndRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		doc      bson.M
		want     bool
		wantKept string // identity_state expected after a Go write-back
	}{
		{"legacy buffer, verdict still in last_signature",
			bson.M{"last_signature": "UNIQUE_OR_PROXY"}, true, ""},
		{"legacy buffer, clobbered to raw hash",
			bson.M{"last_signature": "abc123"}, false, ""},
		{"legacy buffer, clobbered to empty",
			bson.M{"last_signature": ""}, false, ""},
		{"migrated buffer, verdict in its own field",
			bson.M{"last_signature": "abc123", "identity_state": "UNIQUE_OR_PROXY"}, true, "UNIQUE_OR_PROXY"},
		{"migrated buffer, supplier is a duplicate",
			bson.M{"last_signature": "abc123", "identity_state": "IGNORE_OR_DUPLICATED"}, false, "IGNORE_OR_DUPLICATED"},
	}

	for _, c := range cases {
		raw, err := bson.Marshal(c.doc)
		if err != nil {
			t.Fatalf("%s: marshal doc: %v", c.name, err)
		}
		var rec SignatureTaskRecord
		if err := bson.Unmarshal(raw, &rec); err != nil {
			t.Fatalf("%s: unmarshal into record: %v", c.name, err)
		}

		got, err := rec.IsEqual("UNIQUE_OR_PROXY")
		if err != nil {
			t.Fatalf("%s: IsEqual: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: IsEqual = %v, want %v", c.name, got, c.want)
		}

		// What a Go write-back ($set: record) would persist for the field.
		back, err := bson.Marshal(&rec)
		if err != nil {
			t.Fatalf("%s: marshal record: %v", c.name, err)
		}
		var out bson.M
		if err := bson.Unmarshal(back, &out); err != nil {
			t.Fatalf("%s: unmarshal back: %v", c.name, err)
		}
		gotKept, present := out["identity_state"]
		if c.wantKept == "" {
			if present {
				t.Errorf("%s: Go write would ADD identity_state=%v (must stay absent)", c.name, gotKept)
			}
		} else if gotKept != c.wantKept {
			t.Errorf("%s: Go write would persist identity_state=%v, want %v", c.name, gotKept, c.wantKept)
		}
		t.Logf("ok  %-46s IsEqual=%-5v identity_state after write: %v", c.name, got, out["identity_state"])
	}
}
