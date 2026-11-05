package r

import androidx.room.ColumnInfo
import androidx.room.Dao
import androidx.room.Database
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
import androidx.room.RoomDatabase
import androidx.room.Transaction
import androidx.room.TypeConverter
import androidx.room.TypeConverters
import androidx.room.Update
import java.util.Date
import kotlinx.coroutines.flow.Flow

enum class Kind { PERSON, BOT }

data class Address(val street: String, @ColumnInfo(name = "town") val city: String)

@Entity(tableName = "users", indices = [Index(value = ["email"], unique = true)])
data class User(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val name: String,
    val email: String?,
    val age: Int,
    val kind: Kind = Kind.PERSON,
    val score: Double? = null,
    @Embedded(prefix = "addr_") val address: Address?,
)

@Entity(
    foreignKeys = [ForeignKey(entity = User::class, parentColumns = ["id"], childColumns = ["ownerId"], onDelete = ForeignKey.CASCADE)],
    indices = [Index("ownerId")],
)
data class Note(@PrimaryKey val id: String, val ownerId: Long, val text: String, val created: Date?, val tags: List<String>)

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

data class UserWithNotes(
    @Embedded val user: User,
    @Relation(parentColumn = "id", entityColumn = "ownerId") val notes: List<Note>,
)

data class NameAndAge(val name: String, val age: Int)

@Dao
interface UserDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(user: User): Long

    @Insert
    suspend fun insertAll(vararg users: User): List<Long>

    @Update
    suspend fun update(user: User): Int

    @Delete
    suspend fun delete(user: User)

    @Query("SELECT * FROM users WHERE id = :id")
    suspend fun byId(id: Long): User?

    @Query("SELECT * FROM users WHERE age > :min ORDER BY name")
    fun olderThan(min: Int): Flow<List<User>>

    @Query("SELECT name, age FROM users WHERE id IN (:ids)")
    suspend fun names(ids: List<Long>): List<NameAndAge>

    @Transaction
    @Query("SELECT * FROM users")
    suspend fun withNotes(): List<UserWithNotes>

    @Query("SELECT count(*) FROM users WHERE kind = :kind")
    suspend fun count(kind: Kind): Int

    @Query("DELETE FROM users")
    suspend fun clear()
}

@Dao
abstract class NoteDao {
    @Insert
    abstract suspend fun insert(note: Note)

    @Query("SELECT * FROM Note WHERE ownerId = :owner")
    abstract fun forOwner(owner: Long): Flow<List<Note>>

    @Transaction
    open suspend fun replace(owner: Long, notes: List<Note>) {
        notes.forEach { insert(it) }
    }
}

@Database(entities = [User::class, Note::class], version = 1, exportSchema = false)
@TypeConverters(Converters::class)
abstract class AppDb : RoomDatabase() {
    abstract fun users(): UserDao

    abstract fun notes(): NoteDao
}
