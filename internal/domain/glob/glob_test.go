package glob

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		pat, p string
		want   bool
	}{
		{"**/*.go", "a/b/c.go", true},
		{"**/*.go", "c.go", true},
		{"*_test.go", "x/y_test.go", true},
		{"cmd/**", "cmd/loops/main.go", true},
		{"cmd/**", "internal/cmd.go", false},
		{"{dir}/**/*cobertura.xml", "{dir}/a.cobertura.xml", true},
		{"src/[", "src/x", false},
		{"target/scala-*/coverage-report/cobertura.xml", "target/scala-3.9.0/coverage-report/cobertura.xml", true},
	}
	for _, c := range cases {
		if got := Match(c.pat, c.p); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pat, c.p, got, c.want)
		}
	}
	if !Any([]string{"*.py", "*.go"}, "a/b.go") || Any(nil, "a") {
		t.Fatal("Any")
	}
}

func TestValid(t *testing.T) {
	for _, ok := range []string{"**/*.go", "a/b", "*.[ch]pp"} {
		if !Valid(ok) {
			t.Errorf("Valid(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "a/[", "x/\\"} {
		if Valid(bad) {
			t.Errorf("Valid(%q) = true", bad)
		}
	}
}
