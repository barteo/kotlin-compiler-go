package r

import androidx.room.EntityInsertAdapter
import androidx.room.RoomDatabase
import androidx.room.coroutines.createFlow
import androidx.room.util.getColumnIndexOrThrow
import androidx.room.util.performInTransactionSuspending
import androidx.room.util.performSuspending
import androidx.sqlite.SQLiteStatement
import java.util.Date
import javax.`annotation`.processing.Generated
import kotlin.Int
import kotlin.Long
import kotlin.String
import kotlin.Suppress
import kotlin.Unit
import kotlin.collections.List
import kotlin.collections.MutableList
import kotlin.collections.mutableListOf
import kotlin.reflect.KClass
import kotlinx.coroutines.flow.Flow

@Generated(value = ["androidx.room.RoomProcessor"])
@Suppress(names = ["UNCHECKED_CAST", "DEPRECATION", "REDUNDANT_PROJECTION", "REMOVAL"])
public class NoteDao_Impl(
  __db: RoomDatabase,
) : NoteDao() {
  private val __db: RoomDatabase

  private val __insertAdapterOfNote: EntityInsertAdapter<Note>

  private val __converters: Converters = Converters()
  init {
    this.__db = __db
    this.__insertAdapterOfNote = object : EntityInsertAdapter<Note>() {
      protected override fun createQuery(): String = "INSERT OR ABORT INTO `Note` (`id`,`ownerId`,`text`,`created`,`tags`) VALUES (?,?,?,?,?)"

      protected override fun bind(statement: SQLiteStatement, entity: Note) {
        statement.bindText(1, entity.id)
        statement.bindLong(2, entity.ownerId)
        statement.bindText(3, entity.text)
        val _tmpCreated: Date? = entity.created
        val _tmp: Long? = __converters.fromDate(_tmpCreated)
        if (_tmp == null) {
          statement.bindNull(4)
        } else {
          statement.bindLong(4, _tmp)
        }
        val _tmp_1: String = __converters.fromTags(entity.tags)
        statement.bindText(5, _tmp_1)
      }
    }
  }

  public override suspend fun insert(note: Note): Unit = performSuspending(__db, false, true) { _connection ->
    __insertAdapterOfNote.insert(_connection, note)
  }

  public override suspend fun replace(owner: Long, notes: List<Note>): Unit = performInTransactionSuspending(__db) {
    super@NoteDao_Impl.replace(owner, notes)
  }

  public override fun forOwner(owner: Long): Flow<List<Note>> {
    val _sql: String = "SELECT * FROM Note WHERE ownerId = ?"
    return createFlow(__db, false, arrayOf("Note")) { _connection ->
      val _stmt: SQLiteStatement = _connection.prepare(_sql)
      try {
        var _argIndex: Int = 1
        _stmt.bindLong(_argIndex, owner)
        val _columnIndexOfId: Int = getColumnIndexOrThrow(_stmt, "id")
        val _columnIndexOfOwnerId: Int = getColumnIndexOrThrow(_stmt, "ownerId")
        val _columnIndexOfText: Int = getColumnIndexOrThrow(_stmt, "text")
        val _columnIndexOfCreated: Int = getColumnIndexOrThrow(_stmt, "created")
        val _columnIndexOfTags: Int = getColumnIndexOrThrow(_stmt, "tags")
        val _result: MutableList<Note> = mutableListOf()
        while (_stmt.step()) {
          val _item: Note
          val _tmpId: String
          _tmpId = _stmt.getText(_columnIndexOfId)
          val _tmpOwnerId: Long
          _tmpOwnerId = _stmt.getLong(_columnIndexOfOwnerId)
          val _tmpText: String
          _tmpText = _stmt.getText(_columnIndexOfText)
          val _tmpCreated: Date?
          val _tmp: Long?
          if (_stmt.isNull(_columnIndexOfCreated)) {
            _tmp = null
          } else {
            _tmp = _stmt.getLong(_columnIndexOfCreated)
          }
          _tmpCreated = __converters.toDate(_tmp)
          val _tmpTags: List<String>
          val _tmp_1: String
          _tmp_1 = _stmt.getText(_columnIndexOfTags)
          _tmpTags = __converters.toTags(_tmp_1)
          _item = Note(_tmpId,_tmpOwnerId,_tmpText,_tmpCreated,_tmpTags)
          _result.add(_item)
        }
        _result
      } finally {
        _stmt.close()
      }
    }
  }

  public companion object {
    public fun getRequiredConverters(): List<KClass<*>> = emptyList()
  }
}
