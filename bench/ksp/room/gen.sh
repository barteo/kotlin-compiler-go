#!/usr/bin/env bash
# gen.sh writes a Room module of N files into DIR/r, for timing Room's
# processor on more than test/room's one file:
#
#	bench/ksp/room/gen.sh DIR [N]
#
# N (default 100) is the number of ItemNNN.kt files, each with two
# entities (one with an @Embedded class, type-converted columns and a
# unique index; one with a foreign key to it), a @Relation class, a
# projection and a DAO: inserts, updates, deletes, and queries with
# parameters, IN lists, Flow and suspend results and @Transaction; every
# third DAO is an abstract class with an open @Transaction function.
# Common.kt has the shared types and converters, AppDb.kt the @Database
# with every entity and DAO. The output only depends on N.
set -euo pipefail

[ $# -ge 1 ] || { sed -n '2,13p' "$0" >&2; exit 2; }
dir=$1 n=${2:-100}
rm -rf "$dir"
mkdir -p "$dir/r"

cat > "$dir/r/Common.kt" <<'EOF'
package r

import androidx.room.ColumnInfo
import androidx.room.TypeConverter
import java.util.Date

enum class Status { NEW, ACTIVE, DONE }

data class Location(val lat: Double, val lon: Double, @ColumnInfo(name = "place") val name: String?)

class Converters {
    @TypeConverter
    fun fromDate(d: Date?): Long? = d?.time

    @TypeConverter
    fun toDate(v: Long?): Date? = v?.let { Date(it) }

    @TypeConverter
    fun fromTags(tags: List<String>): String = tags.joinToString(",")

    @TypeConverter
    fun toTags(s: String): List<String> = if (s.isEmpty()) emptyList() else s.split(",")
}
EOF

for ((i = 0; i < n; i++)); do
	I=$(printf 'Item%03d' "$i")
	t=$(printf 'item%03d' "$i")
	if ((i % 3 == 2)); then
		head="abstract class ${I}Dao {"
		a="abstract "
	else
		head="interface ${I}Dao {"
		a=""
	fi
	{
		cat <<EOF
package r

import androidx.room.Dao
import androidx.room.Delete
import androidx.room.Embedded
import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Index
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Relation
import androidx.room.Transaction
import androidx.room.Update
import java.util.Date
import kotlinx.coroutines.flow.Flow

@Entity(tableName = "$t", indices = [Index(value = ["code"], unique = true)])
data class $I(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val code: String,
    val title: String?,
    val amount: Int,
    val price: Double? = null,
    val status: Status = Status.NEW,
    val created: Date?,
    val tags: List<String>,
    @Embedded(prefix = "loc_") val location: Location?,
)

@Entity(
    tableName = "${t}_note",
    foreignKeys = [ForeignKey(entity = $I::class, parentColumns = ["id"], childColumns = ["itemId"], onDelete = ForeignKey.CASCADE)],
    indices = [Index("itemId")],
)
data class ${I}Note(@PrimaryKey(autoGenerate = true) val id: Long = 0, val itemId: Long, val text: String, val pinned: Boolean)

data class ${I}WithNotes(
    @Embedded val item: $I,
    @Relation(parentColumn = "id", entityColumn = "itemId") val notes: List<${I}Note>,
)

data class ${I}Summary(val code: String, val amount: Int)

@Dao
$head
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    ${a}suspend fun insert(item: $I): Long

    @Insert
    ${a}suspend fun insertAll(vararg items: $I): List<Long>

    @Insert
    ${a}suspend fun insertNote(note: ${I}Note)

    @Update
    ${a}suspend fun update(item: $I): Int

    @Delete
    ${a}suspend fun delete(item: $I)

    @Query("SELECT * FROM $t WHERE id = :id")
    ${a}suspend fun byId(id: Long): $I?

    @Query("SELECT * FROM $t WHERE status = :status AND amount > :min ORDER BY code")
    ${a}fun byStatus(status: Status, min: Int): Flow<List<$I>>

    @Query("SELECT code, amount FROM $t WHERE id IN (:ids)")
    ${a}suspend fun summaries(ids: List<Long>): List<${I}Summary>

    @Transaction
    @Query("SELECT * FROM $t")
    ${a}suspend fun withNotes(): List<${I}WithNotes>

    @Query("SELECT * FROM ${t}_note WHERE itemId = :item AND pinned = 1")
    ${a}fun pinned(item: Long): Flow<List<${I}Note>>

    @Query("SELECT count(*) FROM $t WHERE created > :since")
    ${a}suspend fun countSince(since: Date): Int

    @Query("DELETE FROM $t")
    ${a}suspend fun clear()
EOF
		if ((i % 3 == 2)); then
			cat <<EOF

    @Transaction
    open suspend fun replace(items: List<$I>) {
        clear()
        items.forEach { insert(it) }
    }
EOF
		fi
		echo "}"
	} > "$dir/r/$I.kt"
done

{
	cat <<'EOF'
package r

import androidx.room.Database
import androidx.room.RoomDatabase
import androidx.room.TypeConverters

@Database(
    entities = [
EOF
	for ((i = 0; i < n; i++)); do
		I=$(printf 'Item%03d' "$i")
		echo "        $I::class, ${I}Note::class,"
	done
	cat <<'EOF'
    ],
    version = 1,
    exportSchema = false,
)
@TypeConverters(Converters::class)
abstract class AppDb : RoomDatabase() {
EOF
	for ((i = 0; i < n; i++)); do
		I=$(printf 'Item%03d' "$i")
		t=$(printf 'item%03d' "$i")
		echo "    abstract fun $t(): ${I}Dao"
	done
	echo "}"
} > "$dir/r/AppDb.kt"
