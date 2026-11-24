package app

import kotlinx.serialization.Serializable

@Serializable
data class Aliased(
    val crate: SCrate<String>,
    val crates: List<SCrate<Int>> = listOf(),
    val maybe: SCrate<String>? = null,
    val chained: IntCrate = CrateImpl(1),
    val nested: SCrate<SCrate<Long>> = CrateImpl(CrateImpl(2L)),
    val nullableArg: SCrate<String?> = CrateImpl(null),
    val wrap: SerializableListWrap<String> = ListWrap(listOf("w")),
    val shout: Shouted = "hey",
    val shouts: List<Shouted> = listOf("a"),
    // The same serializers on the property and on the type as written.
    @Serializable(with = CrateSer::class) val onProp: Crate<Int> = CrateImpl(3),
    val onType: @Serializable(with = CrateSer::class) Crate<Boolean> = CrateImpl(true),
) {
    var body: SCrate<Double> = CrateImpl(0.5)
}

@Serializable
data class GenericAliased<T>(val crate: SCrate<T>, val wrap: SerializableListWrap<T>)

@Serializable
@JvmInline
value class InlineCrate(val c: SCrate<String>)

@Serializable
@JvmInline
value class InlineShout(val s: Shouted)

fun aliases() {
    check(
        "aliased", Aliased.serializer(),
        Aliased(CrateImpl("c"), listOf(CrateImpl(1), CrateImpl(2)), CrateImpl("m"), shout = "loud").apply { body = CrateImpl(1.5) },
    )
    check("aliased defaults", Aliased.serializer(), Aliased(CrateImpl("d")))
    check("generic aliased", GenericAliased.serializer(intSer()), GenericAliased(CrateImpl(4), ListWrap(listOf(5, 6))))
    check("inline crate", InlineCrate.serializer(), InlineCrate(CrateImpl("i")))
    check("inline shout", InlineShout.serializer(), InlineShout("quiet"))
    println(Aliased.serializer().descriptor)
}
