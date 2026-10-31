fun main() {
    var result = 0
    Registry.block = { result = 42 }
    Registry.block()
    println(result)
    try {
        println(Registry.name)
    } catch (e: UninitializedPropertyAccessException) {
        println("uninitialized")
    }
    Registry.name = "registry"
    Registry.count += 2
    Registry.count++
    println("${Registry.name} ${Registry.count}")
    Box.label = "box"
    Box.size = Box.size * 3
    --Box.size
    val b = Box()
    b.s = "s"
    println("${Box.label} ${Box.size} ${Box.fixed} ${b.s}")
}
