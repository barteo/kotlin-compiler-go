package lib

class IdPair(val id: Long)

class Record(val id: Long)

// Overloads with one erasure, which @JvmName tells apart on the JVM.
open class Store {
    @JvmName("deleteWithIdPairs")
    suspend fun delete(pairs: List<IdPair>): Int = pairs.size

    @JvmName("deleteWithRecords")
    suspend fun delete(records: List<Record>): Int = delete(records.map { IdPair(it.id) }) + 100

    @JvmName("countIdPairs")
    fun count(pairs: List<IdPair>): Int = pairs.size

    @JvmName("countRecords")
    fun count(records: List<Record>): Int = records.size + 100
}

class Archive : Store()
