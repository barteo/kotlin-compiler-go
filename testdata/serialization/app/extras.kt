@file:OptIn(ExperimentalUnsignedTypes::class, kotlin.uuid.ExperimentalUuidApi::class, kotlin.time.ExperimentalTime::class)

package app

import kotlinx.serialization.KSerializer
import kotlinx.serialization.Serializable
import kotlinx.serialization.Transient
import kotlinx.serialization.json.Json
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.uuid.Uuid

// More than 32 properties: the synthetic constructor takes a mask for each
// 32, and the missing ones are reported from all of them.
@Serializable
open class Big(
    val p0: Int, val p1: Int = 1, val p2: Int = 2, val p3: Int = 3, val p4: Int = 4,
    val p5: Int = 5, val p6: Int = 6, val p7: Int = 7, val p8: Int = 8, val p9: Int = 9,
    val p10: Int = 10, val p11: Int = 11, val p12: Int = 12, val p13: Int = 13, val p14: Int = 14,
    val p15: Int = 15, val p16: Int = 16, val p17: Int = 17, val p18: Int = 18, val p19: Int = 19,
    val p20: Int = 20, val p21: Int = 21, val p22: Int = 22, val p23: Int = 23, val p24: Int = 24,
    val p25: Int = 25, val p26: Int = 26, val p27: Int = 27, val p28: Int = 28, val p29: Int = 29,
    val p30: Int = 30, val p31: String, val p32: Int = 32, val p33: String = "33", val p34: Long,
) {
    var body: String = "b$p0"
    override fun toString() = "Big($p0, $p1, $p30, $p31, $p32, $p33, $p34, $body)"
}

@Serializable
class Bigger(val q0: Int, val q1: String = "q1") : Big(0, p31 = "s", p34 = 9L) {
    override fun toString() = "Bigger(${super.toString()}, $q0, $q1)"
}

@Serializable
data class Cell(val n: Int)

// Arrays, of primitives, unsigned numbers and objects, and the built-in
// types that kotlinx.serialization has serializers for.
@Serializable
class Arrays(
    val i: IntArray = intArrayOf(1, 2),
    val l: LongArray = longArrayOf(3L),
    val s: ShortArray = shortArrayOf(4),
    val b: ByteArray = byteArrayOf(5),
    val c: CharArray = charArrayOf('c'),
    val f: FloatArray = floatArrayOf(1.5f),
    val d: DoubleArray = doubleArrayOf(2.5),
    val z: BooleanArray = booleanArrayOf(true),
    val ui: UIntArray = uintArrayOf(7u),
    val ul: ULongArray = ulongArrayOf(8u),
    val us: UShortArray = ushortArrayOf(9u),
    val ub: UByteArray = ubyteArrayOf(10u),
    val strs: Array<String> = arrayOf("x", "y"),
    val nullable: Array<String?> = arrayOf(null, "n"),
    val cells: Array<Cell> = arrayOf(Cell(1)),
    val nested: Array<IntArray> = arrayOf(intArrayOf(1), intArrayOf()),
    val boxed: Array<Int> = arrayOf(4, 5),
    val lists: Array<List<String>> = arrayOf(listOf("a")),
    val maybe: IntArray? = null,
    val inList: List<IntArray> = listOf(intArrayOf(6)),
    val dur: Duration = 90.seconds,
    val durN: Duration? = null,
    val uuid: Uuid = Uuid.parse("550e8400-e29b-41d4-a716-446655440000"),
    val at: Instant = Instant.fromEpochSeconds(1700000000),
    val unit: Unit = Unit,
) {
    override fun toString() = "Arrays(${i.toList()}, ${ui.toList()}, ${strs.toList()}, ${nested.map { it.toList() }}, " +
        "${boxed.toList()}, ${maybe?.toList()}, $dur, $uuid, $at, $unit)"
}

// A lateinit property is required; a @Transient one isn't written.
@Serializable
class Late(val id: Int = 0) {
    lateinit var s: String
    var after: String = "a"
    @Transient lateinit var skip: String
    lateinit var list: List<Int>
    init { after += id }
    override fun toString() = "Late($id, $s, $after, $list, ${::skip.isInitialized})"
}

fun <T> roundTrip(name: String, s: KSerializer<T>, v: T) {
    val text = Json.encodeToString(s, v)
    val back = Json.decodeFromString(s, text)
    println("$name: $text")
    println("  back: $back ${Json.encodeToString(s, back) == text}")
}

fun extras() {
    roundTrip("big", Big.serializer(), Big(1, p31 = "x", p34 = 2L, p32 = 7))
    println(Json.decodeFromString(Big.serializer(), """{"p0":3,"p31":"y","p34":4,"body":"z"}"""))
    fails("bigMissing", Big.serializer(), """{"p0":3}""")
    fails("bigMissing2", Big.serializer(), """{"p0":3,"p31":"y"}""")
    roundTrip("bigger", Bigger.serializer(), Bigger(5))
    println(Json.decodeFromString(Bigger.serializer(), """{"p0":1,"p31":"a","p34":2,"q0":3}"""))
    fails("biggerMissing", Bigger.serializer(), """{"p0":1,"p31":"a","q0":3}""")

    roundTrip("arrays", Arrays.serializer(), Arrays())
    println(Json.decodeFromString(Arrays.serializer(),
        """{"i":[9],"strs":["q"],"nested":[[1,2]],"dur":"PT2M","ui":[4294967295],"maybe":[1],"boxed":[1]}"""))
    println(Arrays.serializer().descriptor.getElementDescriptor(12))
    println(Arrays.serializer().descriptor.getElementDescriptor(20).serialName)

    val l = Late(3)
    l.s = "q"
    l.list = listOf(1)
    roundTrip("late", Late.serializer(), l)
    println(Json.decodeFromString(Late.serializer(), """{"s":"z","list":[]}"""))
    fails("lateMissing", Late.serializer(), """{"id":1}""")
    try {
        Json.encodeToString(Late.serializer(), Late())
    } catch (e: UninitializedPropertyAccessException) {
        println("lateUnset: ${e.message}")
    }
    contextual()
}
