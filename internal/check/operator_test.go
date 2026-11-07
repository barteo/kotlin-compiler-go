package check

import (
	"context"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// TestOperatorRequiredExtension checks the error for an indexing call that
// finds a top-level extension 'get' without 'operator', which has no owner
// class to name.
func TestOperatorRequiredExtension(t *testing.T) {
	f, errs := syntax.Parse("t.kt", "class Box\nfun Box.get(i: Int) = i\nfun use(b: Box) = b[0]\n")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	cerrs, err := CheckContext(context.Background(), []*syntax.File{f}, NewLibrary(nil), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "'operator' modifier is required on 'get'"
	for _, e := range cerrs {
		if strings.Contains(e.Error(), want) {
			return
		}
	}
	t.Fatalf("errors %v, want %q", cerrs, want)
}
