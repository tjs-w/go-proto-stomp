package stomp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_escape(t *testing.T) {
	unesc := `foo:1231-123-123:9`
	esc := `foo\c1231-123-123\c9`

	got := escape(unesc)
	t.Log("Escaped:", got, ", Input:", unesc)
	if got != esc {
		t.Error(cmp.Diff(got, esc))
	}

	got = unescape(esc)
	t.Log("Unescaped:", got, ", Input:", esc)
	if got != unesc {
		t.Error(cmp.Diff(got, unesc))
	}
}
