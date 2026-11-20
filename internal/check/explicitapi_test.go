package check

import (
	"slices"
	"testing"
)

const explicitAPISrc = `public interface I {
    public fun i(): Int
}
fun f() = 1
public class C(val p: Int) : I {
    override fun i() = 1
    val x = 2
    private val y = 3
    public constructor() : this(0)
}
public data class D(val d: Int)
internal class E {
    fun e() = 1
}
public fun g() {
    class L {
        fun l() = 1
    }
}
`

func TestExplicitAPI(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
		want []string
	}{
		{"off", Options{Warnings: true}, nil},
		{"strict", Options{Warnings: true, ExplicitAPI: APIStrict},
			// f: visibility and type; p; i: type; x: visibility and type.
			[]string{"4:error", "4:error", "5:error", "6:error", "7:error", "7:error"}},
		{"warning", Options{Warnings: true, ExplicitAPI: APIWarning}, []string{
			"4:NO_EXPLICIT_RETURN_TYPE_IN_API_MODE_WARNING", "4:NO_EXPLICIT_VISIBILITY_IN_API_MODE_WARNING",
			"5:NO_EXPLICIT_VISIBILITY_IN_API_MODE_WARNING",
			"6:NO_EXPLICIT_RETURN_TYPE_IN_API_MODE_WARNING",
			"7:NO_EXPLICIT_RETURN_TYPE_IN_API_MODE_WARNING", "7:NO_EXPLICIT_VISIBILITY_IN_API_MODE_WARNING"}},
		{"return types", Options{Warnings: true, ExplicitReturnTypes: APIStrict}, []string{"4:error", "6:error", "7:error"}},
	} {
		got := checkWarningsWith(t, explicitAPISrc, tc.opts)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
