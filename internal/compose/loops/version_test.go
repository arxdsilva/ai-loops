package loops

import "testing"

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]string{
		`{"version":"1.2.3"}`: "1.2.3",
		`{"version":""}`:      DevVersion,
		`{}`:                  DevVersion,
		`{bad`:                DevVersion,
	} {
		if got := parseVersion([]byte(in)); got != want {
			t.Errorf("%s: %q want %q", in, got, want)
		}
	}
}

func TestEmbeddedVersionIsPluginJSON(t *testing.T) {
	if v := embeddedVersion(); v == DevVersion || v == "" {
		t.Fatalf("embedded version = %q", v)
	}
}
