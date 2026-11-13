import lib.Archive
import lib.IdPair
import lib.Record
import lib.Store

suspend fun main() {
    val store = Store()
    val records = listOf(Record(1), Record(2))
    println(store.count(records))
    println(store.count(listOf(IdPair(1))))
    println(store.delete(records))
    println(store.delete(listOf(IdPair(1))))
    println(Archive().count(records) + Archive().delete(listOf(IdPair(3))))
}
