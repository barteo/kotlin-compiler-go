package ka

import (
	"slices"
	"strings"
	"testing"
)

func TestPackage(t *testing.T) {
	m := update(t, "package p\nclass A")
	defer m.Close()
	if p := m.Package("kotlin.no.such"); p != nil {
		t.Errorf("kotlin.no.such: %+v", p)
	}
	if p := m.Package("kotlin"); p == nil || !slices.Contains(p.Classes, "kotlin/Deprecated") || !slices.Contains(p.Classes, "kotlin/Int") {
		t.Errorf("kotlin: %+v", p)
	}
	if p := m.Package("kotlin.collections"); p != m.Package("kotlin.collections") {
		t.Error("Package isn't cached")
	}
	// The root package has packages in it, and the sources' packages are
	// left out.
	if p := m.Package(""); p == nil || len(p.Classes) != 0 || len(p.Decls) != 0 {
		t.Errorf("root: %+v", p)
	}
	if p := m.Package("p"); p != nil {
		t.Errorf("source package p: %+v", p)
	}
	p := m.Package("kotlin.collections")
	if p == nil {
		t.Fatal("kotlin.collections: nil")
	}
	for _, id := range []string{"kotlin/collections/AbstractList", "kotlin/collections/ArrayDeque", "kotlin/collections/List", "kotlin/collections/MutableMap"} {
		if !slices.Contains(p.Classes, id) {
			t.Errorf("no class %s", id)
		}
	}
	for _, id := range p.Classes {
		if strings.HasSuffix(id, "Kt") || strings.Contains(id, "__") || strings.Contains(id, ".") {
			t.Errorf("class %s is a facade or nested", id)
		}
	}
	var sb strings.Builder
	for _, d := range p.Decls {
		switch {
		case d.Name == "lastIndex" && d.Receiver.ClassID != "kotlin/collections/List":
		case d.Name == "emptyList", d.Name == "arrayListOf", d.Name == "lastIndex", d.Name == "ArrayList", d.Name == "LinkedHashMap":
			dumpLibrary(&sb, d, "", nil)
		}
	}
	want := `7 arrayListOf [kotlin/collections/CollectionsKt__CollectionsKt] v0 m0 f0x80 @kotlin/SinceKotlin(version = "1.1") @kotlin/internal/InlineOnly() <T>: java/util/ArrayList<T> (as kotlin/collections/ArrayList<T>)
7 arrayListOf [kotlin/collections/CollectionsKt__CollectionsKt] v0 m0 f0x0 <T> (elements: T): java/util/ArrayList<T> (as kotlin/collections/ArrayList<T>)
7 emptyList [kotlin/collections/CollectionsKt__CollectionsKt] v0 m0 f0x0 <T>: kotlin/collections/List<T>
8 lastIndex [kotlin/collections/CollectionsKt__CollectionsKt] v0 m0 f0x0 <T> recv kotlin/collections/List<T>: kotlin/Int
9 ArrayList [kotlin/collections/ArrayList] v0 m0 f0x0 <E>: java/util/ArrayList<E>
9 LinkedHashMap [kotlin/collections/LinkedHashMap] v0 m0 f0x0 <K> <V>: java/util/LinkedHashMap<K, V>
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAbbreviations(t *testing.T) {
	m := updateWith(t, withJDK(t), `package p
typealias S = String
typealias L<T> = List<T>
typealias SS = S
class C(val a: S?, val b: L<S>, val c: SS, val d: ArrayList<Int>, val e: List<S>)
`, `package q
import java.util.*
class D(val f: ArrayList<Int>, val g: kotlin.collections.ArrayList<Int>?, val h: Exception, val i: java.lang.Exception)
`, `package r
import java.util.ArrayList
class E(val j: ArrayList<Int>, val k: HashMap<String, Int>)
`)
	defer m.Close()
	var sb strings.Builder
	for _, d := range slices.Concat(m.Files[0].Decls, m.Files[1].Decls, m.Files[2].Decls) {
		if d.Kind == KindTypeAlias {
			sb.WriteString(d.Name + " = " + libType(d.Type) + "\n")
			continue
		}
		for _, md := range d.Members {
			if md.Kind == KindProperty {
				sb.WriteString(md.Name + ": " + libType(md.Type) + "\n")
			}
		}
	}
	want := `S = kotlin/String
L = kotlin/collections/List<T>
SS = kotlin/String (as p/S)
a: kotlin/String? (as p/S?)
b: kotlin/collections/List<kotlin/String (as p/S)> (as p/L<kotlin/String (as p/S)>)
c: kotlin/String (as p/SS)
d: java/util/ArrayList<kotlin/Int> (as kotlin/collections/ArrayList<kotlin/Int>)
e: kotlin/collections/List<kotlin/String (as p/S)>
f: java/util/ArrayList<kotlin/Int>
g: java/util/ArrayList<kotlin/Int>? (as kotlin/collections/ArrayList<kotlin/Int>?)
h: java/lang/Exception (as kotlin/Exception)
i: java/lang/Exception
j: java/util/ArrayList<kotlin/Int>
k: java/util/HashMap<kotlin/String, kotlin/Int> (as kotlin/collections/HashMap<kotlin/String, kotlin/Int>)
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
