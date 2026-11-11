class Data(val values: Map<String, String>?)

fun Data.statement(): String {
    requireNotNull(values)
    return values["id"]!!
}

fun Data.chained(): String = requireNotNull(values).let { values["id"]!! }

fun Data.withMessage(): String = requireNotNull(values) { "values are required" }.let { values["id"]!! }

fun param(m: Map<String, String>?): String = checkNotNull(m).let { m["id"]!! }

fun main() {
    val d = Data(mapOf("id" to "v"))
    println(d.statement() + d.chained() + d.withMessage() + param(d.values))
}
