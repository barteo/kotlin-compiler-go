open class Table(val name: String)

open class Column(val column: String, val table: Table) {
    override fun toString() = "${table.name}.$column"
}

object Users : Table("users") {
    object Id : Column("id", this)
    object Name : Column("name", this)
    class Custom(column: String) : Column(column, this)
}

class Host {
    companion object : Table("host") {
        object All : Column("*", this)
    }
}

fun main() {
    println(Users.Id)
    println(Users.Name)
    println(Users.Custom("email"))
    println(Host.Companion.All)
}
