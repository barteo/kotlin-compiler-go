// Extension functions declared in interfaces and open classes: abstract,
// with bodies, overridden, and called with an implicit receiver as the
// dispatch receiver, as Exposed's DSL does.

class Column<T>(val name: String)
class Op(val sql: String) { override fun toString() = sql }

interface ExpressionBuilder {
    infix fun <T> Column<T>.eq(value: T): Op = Op("$name = $value")
    infix fun <T> Column<T>.neq(value: T): Op
    fun Column<*>.isNull(): Op = Op("$name IS NULL")
}

object SqlBuilder : ExpressionBuilder {
    override fun <T> Column<T>.neq(value: T): Op = Op("$name <> $value")
}

open class Formatter {
    open fun Int.show(): String = "int $this"
    fun format(x: Int) = x.show()
}

class Hex : Formatter() {
    override fun Int.show(): String = "hex " + Integer.toHexString(this)
}

abstract class Shape {
    abstract fun Double.scaled(): Double
    fun area(r: Double) = r.scaled() * 3
}

class Doubler : Shape() {
    override fun Double.scaled() = this * 2
}

fun where(f: ExpressionBuilder.() -> Op): Op = SqlBuilder.f()

class Query : ExpressionBuilder {
    override fun <T> Column<T>.neq(value: T): Op = Op("NOT ($name = $value)")
    fun build(c: Column<Int>) = c neq 3
    fun nested(c: Column<Int>) = listOf(1, 2).map { c eq it }
}

// Overloads whose receivers mention the class's type parameter: as the
// receiver, the List one is more specific, as in Arrow's Raise<Error>.
class Box<out E, out A>(val e: E, val a: A)

interface Binder<Error> {
    fun <A> Iterable<Box<Error, A>>.bindAll(): String = "iterable"
    fun <A> List<Box<Error, A>>.bindAll(): String = "list"
}

object StringBinder : Binder<String>

fun <Error, A> List<Box<Error, A>>.viaWith(b: Binder<Error>): String = with(b) { bindAll() }

fun main() {
    println(listOf(Box("e", 1)).viaWith(StringBinder))
    val id = Column<Int>("id")
    val name = Column<String>("name")
    println(with(SqlBuilder) { id eq 1 })
    println(where { name neq "x" })
    println(where { id.isNull() })
    println(Formatter().format(255) + " / " + Hex().format(255))
    val f: Formatter = Hex()
    println(f.format(10))
    println(Doubler().area(1.5))
    val q = Query()
    println(q.build(id))
    println(q.nested(id))
    println(with(q) { id neq 4 })
}
