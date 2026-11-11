// An operator whose right operand is a lambda takes its type from the
// operator function: 2 * { mark("x") } for Int.times(block: Scope.() -> Unit).
class Scope { val log = mutableListOf<String>(); fun mark(s: String) { log += s } }
class CountScope(val scope: Scope) {
    operator fun Int.times(block: Scope.() -> Unit) { repeat(this) { scope.block() } }
}
fun count(block: CountScope.() -> Unit): List<String> { val s = Scope(); CountScope(s).block(); return s.log }
fun main() { println(count { 2 * { mark("x") }; 1 * { mark("y") } }) }
