package ide

import "testing"

const hoverSrc = `package p

/**
 * A box of [T].
 *
 * @param T the item's type
 * @property item what it holds
 */
open class Box<T>(val item: T) {
    /** Returns [item]. */
    open fun get(): T = item

    @Suppress("x")
    internal operator fun plus(other: Box<T>) = this
}

fun use(b: Box<String>?, xs: List<Int>): Int {
    val n = xs.size
    val m = xs + 1
    val f = { s: String -> s.length }
    if (b != null) println(b.item)
    return n + f("a") + (b?.plus(b)?.get()?.length ?: 0)
}
`

func TestHover(t *testing.T) {
	v := testView(t, "a.kt", hoverSrc)
	for _, c := range []struct{ at, want string }{
		{"class B|ox", "```kotlin\nopen class Box<T>(val item: T)\n```\n\nA box of `T`.\n\n**Params:**\n- `T` the item's type\n\n**Properties:**\n- `item` what it holds\n\npackage `p`"},
		{"open fun g|et", "```kotlin\nopen fun get(): T\n```\n\nReturns `item`.\n\nmember of `Box`"},
		{"fun pl|us", "```kotlin\ninternal operator fun plus(other: Box<T>): Box<T>\n```\n\nmember of `Box`"},
		{"val |n", "```kotlin\nval n: Int\n```"},
		{"val |f", "```kotlin\nval f: (String) -> Int\n```"},
		{"xs.si|ze", "```kotlin\nval size: Int\n```"},
		{"print|ln", "```kotlin\ninline fun println(message: Any?): Unit\n```"},
		{"println(|b.item)", "```kotlin\nb: Box<String>?\n```\n\nSmart cast to `Box<String>`"},
		{"xs |+ 1", "```kotlin\noperator fun <T> Collection<T>.plus(element: T): List<T>\n```"},
		{"?.g|et()", "```kotlin\nopen fun get(): T\n```\n\nReturns `item`.\n\nmember of `Box`"},
	} {
		off := caret(t, hoverSrc, c.at)
		got, _, ok := Hover(v, "a.kt", off)
		if !ok || got != c.want {
			t.Errorf("hover at %q:\n%s\nwant\n%s", c.at, got, c.want)
		}
	}
	for _, at := range []string{"|{ s", "n |+ f"} {
		if got, _, ok := Hover(v, "a.kt", caret(t, hoverSrc, at)); ok {
			t.Errorf("hover at %q: %s", at, got)
		}
	}
}

func TestKDoc(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"/** One line. */", "One line."},
		{"/**\n * Text with [link] and [md](http://x) and `[code]`.\n *\n * ```\n * @notatag\n * ```\n * @return the\n *   result\n * @throws IllegalStateException when [x]\n * @since 1.2\n */",
			"Text with `link` and [md](http://x) and `[code]`.\n\n```\n@notatag\n```\n\n**Returns:**\n- the result\n\n**Throws:**\n- `IllegalStateException` when `x`\n\n**Since:**\n- 1.2"},
		{"/** @see [Other] */", "**See also:**\n- `Other`"},
	} {
		if got := renderKDoc(c.in); got != c.want {
			t.Errorf("renderKDoc(%q):\n%s\nwant\n%s", c.in, got, c.want)
		}
	}
}
