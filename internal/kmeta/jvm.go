package kmeta

import (
	"strconv"
	"strings"
)

// classMap maps Kotlin class ids to JVM descriptors, as ClassMapperLite
// does. Metadata omits a JVM signature when this mapping gives it.
var classMap = func() map[string]string {
	m := map[string]string{"kotlin/Unit": "V"}
	prims := []string{
		"Boolean", "Z",
		"Char", "C",
		"Byte", "B",
		"Short", "S",
		"Int", "I",
		"Float", "F",
		"Long", "J",
		"Double", "D",
	}
	for i := 0; i < len(prims); i += 2 {
		m["kotlin/"+prims[i]] = prims[i+1]
		m["kotlin/"+prims[i]+"Array"] = "[" + prims[i+1]
	}
	add := func(kotlin, java string) { m["kotlin/"+kotlin] = "L" + java + ";" }
	add("Any", "java/lang/Object")
	add("Nothing", "java/lang/Void")
	add("Annotation", "java/lang/annotation/Annotation")
	for _, c := range []string{"String", "CharSequence", "Throwable", "Cloneable", "Number", "Comparable", "Enum"} {
		add(c, "java/lang/"+c)
	}
	for _, c := range []string{"Iterator", "Collection", "List", "Set", "Map", "ListIterator"} {
		add("collections/"+c, "java/util/"+c)
		add("collections/Mutable"+c, "java/util/"+c)
	}
	add("collections/Iterable", "java/lang/Iterable")
	add("collections/MutableIterable", "java/lang/Iterable")
	add("collections/Map.Entry", "java/util/Map$Entry")
	add("collections/MutableMap.MutableEntry", "java/util/Map$Entry")
	for i := 0; i <= 22; i++ {
		n := strconv.Itoa(i)
		add("Function"+n, "kotlin/jvm/functions/Function"+n)
		add("reflect/KFunction"+n, "kotlin/reflect/KFunction")
	}
	for _, c := range []string{"Char", "Byte", "Short", "Int", "Float", "Long", "Double", "String", "Enum"} {
		add(c+".Companion", "kotlin/jvm/internal/"+c+"CompanionObject")
	}
	return m
}()

// MapClass returns the JVM descriptor ClassMapperLite gives a class id.
func MapClass(id string) string {
	if d, ok := classMap[id]; ok {
		return d
	}
	return "L" + strings.ReplaceAll(id, ".", "$") + ";"
}

// mapTypeDefault returns the descriptor of a type as metadata assumes it
// when it omits a signature, or "" if the type has no class.
func mapTypeDefault(t *Type) string {
	if t.Class == "" {
		return ""
	}
	return MapClass(t.Class)
}

// defaultDesc returns the method descriptor metadata assumes when it omits
// one, or "" if it can't be computed.
func defaultDesc(params []*Type, ret *Type) string {
	var b strings.Builder
	b.WriteByte('(')
	for _, p := range params {
		d := mapTypeDefault(p)
		if d == "" {
			return ""
		}
		b.WriteString(d)
	}
	b.WriteByte(')')
	d := mapTypeDefault(ret)
	if d == "" {
		return ""
	}
	b.WriteString(d)
	return b.String()
}
