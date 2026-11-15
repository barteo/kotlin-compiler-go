// Named arguments of Java methods: only those overriding Kotlin's
// collection members take them, by the Kotlin names. And a Java
// annotation's Class elements are KClasses.

fun use(j: J, m: java.util.ArrayList<String>, a: MyAnn) {
    j.put(key = "k", value = 1)
    j.getOrDefault(key = "k", defaultValue = 2)
    m.add(element = "x")
    m.add(index = 0, element = "y")
    j.f(p = 1) {} // ERROR named arguments are not allowed
    val k: List<Class<*>> = a.value.map { it.java }
}
