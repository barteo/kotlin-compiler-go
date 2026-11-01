// Smart casts that follow from other facts than a direct check of the
// value, as K2 makes them: a function's contract, a val's value, a when's
// branches, a Boolean val, and chains of stable properties.
import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

sealed class Method(open val name: String?) {
    data class Bank(override val name: String, val direct: Direct?) : Method(name)
    data class Card(override val name: String) : Method(name)
    data class Direct(val bank: String, val sort: String)
}

@OptIn(ExperimentalContracts::class)
fun isBank(m: Method): Boolean {
    contract { returns(true) implies (m is Method.Bank) }
    return m is Method.Bank && m.direct != null
}

@OptIn(ExperimentalContracts::class)
fun String?.present(): Boolean {
    contract { returns(true) implies (this@present != null) }
    return this != null
}

@OptIn(ExperimentalContracts::class)
fun requireBank(m: Method) {
    contract { returns() implies (m is Method.Bank) }
    require(m is Method.Bank)
}

sealed class Result<out E, out A> {
    data class Failure<E>(val e: E) : Result<E, Nothing>()
    data class Success<A>(val a: A) : Result<Nothing, A>()
}

// The type of an is in a contract can leave out the type arguments that
// the parameter's type gives, as Arrow's this@bind is Either.Right.
@OptIn(ExperimentalContracts::class)
fun <E, A> Result<E, A>.requireSuccess() {
    contract { returns() implies (this@requireSuccess is Result.Success) }
    require(this is Result.Success)
}

fun contracts(ms: List<Method>) {
    for (m in ms) {
        if (isBank(m)) {
            requireNotNull(m.direct)
            println(m.direct.bank + m.direct.sort)
        } else {
            println("not bank: ${m.name}")
        }
    }
    val s: String? = "hi"
    if (s.present()) println(s.length)
    val first = ms[0]
    requireBank(first)
    println(first.direct)
    val r: Result<String, Int> = Result.Success(4)
    r.requireSuccess()
    println((r as Result.Success).a + 1)
}

class Locked(val text: String?, val icon: String)
class Item(val locked: Locked?)

// val t = a?.b: t not null means a isn't.
fun safeCallVal(item: Item): String {
    val title = item.locked?.text?.let { "<$it>" }
    return title?.let { it + item.locked.icon } ?: "none"
}

fun safeCallValCheck(item: Item): String {
    val text = item.locked?.text
    if (text != null) return text + item.locked.icon
    return "-"
}

enum class Act { A, B }
class Btn(val action: Act?, val title: String)
class Tag(val tier: Act?, val text: String)
class Row(val action: Act?, val name: String?)

// A subject equal to a value that isn't null isn't null.
fun whenEquals(button: Btn?) = when (button?.action) {
    Act.A -> button.title
    Act.B -> button.title + "!"
    null -> "none"
}

fun whenSubjectVal(r: Row) = when (val a = r.action) {
    Act.A -> a.name
    else -> "x"
}

// After a when whose null branch jumps, the subject isn't null.
fun afterWhen(t: Tag?): String? {
    val tier = when (t?.tier) {
        Act.A -> 1
        Act.B -> 2
        null -> return null
    }
    return t.text + tier
}

class Grace(val icon: String)

fun elvisJump(g: Grace?): String? {
    g ?: return null
    return g.icon
}

class Total(val v: Int)
class Header(val total: Total?)
class ViewData(val header: Header)

// A Boolean val's value, of a chain of stable properties.
fun booleanVal(vd: ViewData): Int {
    val showHeader = vd.header.total != null
    if (showHeader) return vd.header.total.v
    return -1
}

fun booleanValAnd(a: String?, b: String?): String? {
    val ok = a != null && b != null && a.isNotEmpty()
    return if (ok) a + b.length else null
}

fun booleanValContract(id: String?): Int? {
    val valid = id.orEmpty().length > 0 && !id.isNullOrEmpty()
    return if (valid) id.length else null
}

interface Action
sealed interface Step {
    data class A(val x: Int) : Step, Action
    data class B(val y: Int) : Step
}
class External(val step: Step)

// A smart cast to an interface the declared type doesn't implement keeps
// the declared type too.
fun intersection(steps: List<Step>): External? {
    val s = steps[0]
    if (s is Action) return External(s)
    return null
}

sealed class Msg {
    sealed class Resp : Msg() {
        class A : Resp()
        class B : Resp()
    }
    class Other : Msg()
}

fun single(r: Msg.Resp) = "resp"

// is A, is B: the common supertype of A and B.
fun multiIs(m: Msg) = when (m) {
    is Msg.Resp.A, is Msg.Resp.B -> single(m)
    else -> "other"
}

sealed class Pin(private val back: Boolean) {
    val isBack = back

    // A private property isn't inherited: this one hides nothing.
    data class Enter(val back: Boolean = true) : Pin(back)
}

sealed class Request {
    data class Create(val category: String, val date: Int) : Request()
    object None : Request()
}

// The smart casts of a when's branch hold in the lambdas of its value,
// which are checked with the other branches'.
fun branchLambda(r: Request): List<String> {
    val items = when (r) {
        is Request.Create -> sequence { yield(r.category + r.date) }
        else -> sequenceOf("none")
    }
    return items.toList()
}

fun main() {
    contracts(listOf(Method.Bank("b", Method.Direct("x", "1")), Method.Card("c")))
    println(safeCallVal(Item(Locked("t", "i"))) + safeCallVal(Item(null)) + safeCallValCheck(Item(Locked("a", "b"))))
    println(whenEquals(Btn(Act.A, "t")) + whenEquals(null) + whenSubjectVal(Row(Act.A, null)))
    println(afterWhen(Tag(Act.B, "x")) + afterWhen(null) + elvisJump(Grace("i")) + elvisJump(null))
    println(booleanVal(ViewData(Header(Total(3)))) + booleanVal(ViewData(Header(null))))
    println(booleanValAnd("a", "bc") + booleanValAnd(null, "x"))
    println(booleanValContract("xyz"))
    println(intersection(listOf(Step.A(1)))?.step)
    println(multiIs(Msg.Resp.A()) + multiIs(Msg.Other()))
    println(Pin.Enter(false).isBack)
    println(branchLambda(Request.Create("c", 1)) + branchLambda(Request.None))
}
