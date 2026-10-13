package routing

import "testing"

func TestPathPrefixMatchesWholeSegments(t *testing.T) {
	for _, tc := range []struct {
		prefix, path string
		want bool
	}{
		{"/", "/", true},
		{"/", "/api", true},
		{"/admin", "/admin", true},
		{"/admin", "/admin/settings", true},
		{"/admin", "/administrator", false},
		{"/admin/", "/admin/settings", true},
		{"/api", "/apis", false},
		{"/api", "/api/v1", true},
	} {
		if got := PathPrefixMatches(tc.prefix, tc.path); got != tc.want {
			t.Errorf("prefix=%q path=%q got=%v want=%v", tc.prefix, tc.path, got, tc.want)
		}
	}
}
