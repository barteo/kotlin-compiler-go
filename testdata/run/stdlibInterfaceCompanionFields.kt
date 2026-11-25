// The properties of an interface's and an annotation class's companion,
// whose private static fields the companion holds and sets in its own
// static initializer, and of an interface's companion whose properties are
// all @JvmField ones, which the interface holds.

import java.lang.reflect.Modifier

interface Api {
    companion object {
        val names: List<String> = listOf("a", "b")
        var count: Int = 0
        private var secret: Long = 10L
        var doubled: Int = 1
            get() = field * 2
            set(v) { field = v + 1 }
        val lazyName: String by lazy { "lazy-" + names.size }
        lateinit var late: String
        lateinit var privateSet: String
            private set
        @JvmStatic val st: Int = 5
        const val C = 3

        init {
            count = names.size
            println("Api.Companion init $count")
        }

        fun bump(): Long {
            count++
            secret += 5
            privateSet = "private set"
            return secret
        }

        fun reader(): () -> String = { "$count:$secret" }
    }

    fun read() = "$count $doubled $lazyName"
}

interface Fields {
    companion object {
        @JvmField val first: String = "first"
        @JvmField val second: String = first + "-second"
    }
}

annotation class Tag {
    companion object {
        val label: String = "tag"
        var uses: Int = 0
    }
}

class Impl : Api

fun fields(c: Class<*>, names: List<String>) {
    for (name in names) {
        val f = c.getDeclaredField(name)
        println("${c.simpleName}.$name ${Modifier.toString(f.modifiers)}")
    }
}

fun main() {
    println(Api.bump())
    println(Impl().read())
    Api.doubled = 4
    Api.late = "late"
    println("${Api.doubled} ${Api.late} ${Api.privateSet} ${Api.st} ${Api.C}")
    println(Api.reader()())
    Api.count -= 1
    println(Impl().read())
    println(Fields.first + " " + Fields.second)
    Tag.uses += 2
    println(Tag.label + " " + Tag.uses)
    fields(Api.Companion::class.java, listOf("names", "count", "secret", "doubled", "lazyName\$delegate", "late", "privateSet", "st"))
    fields(Fields::class.java, listOf("first", "second"))
    fields(Tag.Companion::class.java, listOf("label", "uses"))
}
