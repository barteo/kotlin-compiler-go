package check

import (
	"sync"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/inline"
)

func TestConcurrentInlineCache(t *testing.T) {
	stdlib, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open("", []string{stdlib})
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	table := newClassTable(cp)
	const class = "kotlin/StandardKt__StandardKt"
	const name = "let"
	const desc = "(Ljava/lang/Object;Lkotlin/jvm/functions/Function1;)Ljava/lang/Object;"
	bodies := make([]*inline.Body, 32)
	var wg sync.WaitGroup
	for i := range bodies {
		wg.Go(func() {
			var err error
			bodies[i], err = table.loadInline(class)(name, desc)
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	for _, b := range bodies {
		if b == nil || b != bodies[0] {
			t.Fatal("inline body was not shared")
		}
	}
	if n, _ := cp.Inline.Get(); n != 1 {
		t.Errorf("analyzed %d bodies, want 1", n)
	}
}
