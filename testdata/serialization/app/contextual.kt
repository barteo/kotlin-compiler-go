@file:UseSerializers(DateSer::class, BoxSer::class)
@file:UseContextualSerialization(java.util.UUID::class, Tagged::class)

package app

import kotlinx.serialization.Contextual
import kotlinx.serialization.KSerializer
import kotlinx.serialization.Polymorphic
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.UseContextualSerialization
import kotlinx.serialization.UseSerializers
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.Json
import kotlinx.serialization.modules.SerializersModule
import kotlinx.serialization.modules.contextual
import kotlinx.serialization.modules.polymorphic
import kotlinx.serialization.modules.subclass

// The serializers of a file's @UseSerializers, and @Contextual and
// @Polymorphic properties and types, whose serializers the format's
// module gives.
object DateSer : KSerializer<java.util.Date> {
    override val descriptor = PrimitiveSerialDescriptor("D", PrimitiveKind.LONG)
    override fun serialize(encoder: Encoder, value: java.util.Date) = encoder.encodeLong(value.time)
    override fun deserialize(decoder: Decoder) = java.util.Date(decoder.decodeLong())
}

class Boxed(val s: String)

class BoxSer : KSerializer<Boxed> {
    override val descriptor = PrimitiveSerialDescriptor("Boxed", PrimitiveKind.STRING)
    override fun serialize(encoder: Encoder, value: Boxed) = encoder.encodeString("<" + value.s + ">")
    override fun deserialize(decoder: Decoder) = Boxed(decoder.decodeString().trim('<', '>'))
}

class Money(val v: Long)

object MoneySer : KSerializer<Money> {
    override val descriptor = PrimitiveSerialDescriptor("Money", PrimitiveKind.LONG)
    override fun serialize(encoder: Encoder, value: Money) = encoder.encodeLong(value.v)
    override fun deserialize(decoder: Decoder) = Money(decoder.decodeLong())
}

@Serializable
data class Tagged(val tag: String)

interface Critter

@Serializable
@SerialName("cat")
data class Kitty(val lives: Int) : Critter

@Serializable
abstract class Figure

@Serializable
@SerialName("sq")
data class Square(val side: Int) : Figure()

@Serializable
class Wallet(
    @Contextual val m: Money,
    val l: List<@Contextual Money>,
    val mm: Map<String, @Contextual Money?> = mapOf(),
    val d: java.util.Date,
    val dl: List<java.util.Date> = listOf(),
    val box: Boxed = Boxed("b"),
    val u: java.util.UUID,
    val t: Tagged = Tagged("x"),
    @Polymorphic val a: Critter? = null,
    @Polymorphic val sh: Figure = Square(2),
    @Contextual val ci: Int = 5,
    @Contextual val ct: Tagged? = null,
) {
    override fun toString() = "Wallet(${m.v}, ${l.map { it.v }}, ${d.time}, ${box.s}, $u, $t, $a, $sh, $ci, $ct)"
}

object UuidSer : KSerializer<java.util.UUID> {
    override val descriptor = PrimitiveSerialDescriptor("U", PrimitiveKind.STRING)
    override fun serialize(encoder: Encoder, value: java.util.UUID) = encoder.encodeString(value.toString())
    override fun deserialize(decoder: Decoder) = java.util.UUID.fromString(decoder.decodeString())
}

fun contextual() {
    val module = SerializersModule {
        contextual(MoneySer)
        contextual(java.util.UUID::class, UuidSer)
        polymorphic(Critter::class) { subclass(Kitty::class) }
        polymorphic(Figure::class) { subclass(Square::class) }
        contextual(Int::class, Int.serializer())
    }
    val json = Json { serializersModule = module; encodeDefaults = true }
    val w = Wallet(Money(5), listOf(Money(1)), mapOf("k" to null), java.util.Date(1000), listOf(java.util.Date(2)),
        Boxed("q"), java.util.UUID(1, 2), Tagged("y"), Kitty(9), Square(3), 7, Tagged("z"))
    val text = json.encodeToString(Wallet.serializer(), w)
    println("wallet: $text")
    val back = json.decodeFromString(Wallet.serializer(), text)
    println("  back: $back ${json.encodeToString(Wallet.serializer(), back) == text}")
    println(Wallet.serializer().descriptor.getElementDescriptor(0).kind)
    println(Wallet.serializer().descriptor.getElementDescriptor(8).serialName)
    try {
        Json.encodeToString(Wallet.serializer(), w)
    } catch (e: SerializationException) {
        println("noModule: ${e.message?.lines()?.first()}")
    }
}
