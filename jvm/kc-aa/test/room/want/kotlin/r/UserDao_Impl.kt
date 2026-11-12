package r

import androidx.collection.LongSparseArray
import androidx.room.EntityDeleteOrUpdateAdapter
import androidx.room.EntityInsertAdapter
import androidx.room.RoomDatabase
import androidx.room.coroutines.createFlow
import androidx.room.util.appendPlaceholders
import androidx.room.util.getColumnIndex
import androidx.room.util.getColumnIndexOrThrow
import androidx.room.util.performSuspending
import androidx.room.util.recursiveFetchLongSparseArray
import androidx.sqlite.SQLiteConnection
import androidx.sqlite.SQLiteStatement
import java.util.Date
import javax.`annotation`.processing.Generated
import kotlin.Double
import kotlin.IllegalArgumentException
import kotlin.Int
import kotlin.Long
import kotlin.String
import kotlin.Suppress
import kotlin.Unit
import kotlin.collections.List
import kotlin.collections.MutableList
import kotlin.collections.mutableListOf
import kotlin.reflect.KClass
import kotlin.text.StringBuilder
import kotlinx.coroutines.flow.Flow

@Generated(value = ["androidx.room.RoomProcessor"])
@Suppress(names = ["UNCHECKED_CAST", "DEPRECATION", "REDUNDANT_PROJECTION", "REMOVAL"])
public class UserDao_Impl(
  __db: RoomDatabase,
) : UserDao {
  private val __db: RoomDatabase

  private val __insertAdapterOfUser: EntityInsertAdapter<User>

  private val __insertAdapterOfUser_1: EntityInsertAdapter<User>

  private val __deleteAdapterOfUser: EntityDeleteOrUpdateAdapter<User>

  private val __updateAdapterOfUser: EntityDeleteOrUpdateAdapter<User>

  private val __converters: Converters = Converters()
  init {
    this.__db = __db
    this.__insertAdapterOfUser = object : EntityInsertAdapter<User>() {
      protected override fun createQuery(): String = "INSERT OR REPLACE INTO `users` (`id`,`name`,`email`,`age`,`kind`,`score`,`addr_street`,`addr_town`) VALUES (nullif(?, 0),?,?,?,?,?,?,?)"

      protected override fun bind(statement: SQLiteStatement, entity: User) {
        statement.bindLong(1, entity.id)
        statement.bindText(2, entity.name)
        val _tmpEmail: String? = entity.email
        if (_tmpEmail == null) {
          statement.bindNull(3)
        } else {
          statement.bindText(3, _tmpEmail)
        }
        statement.bindLong(4, entity.age.toLong())
        statement.bindText(5, __Kind_enumToString(entity.kind))
        val _tmpScore: Double? = entity.score
        if (_tmpScore == null) {
          statement.bindNull(6)
        } else {
          statement.bindDouble(6, _tmpScore)
        }
        val _tmpAddress: Address? = entity.address
        if (_tmpAddress != null) {
          statement.bindText(7, _tmpAddress.street)
          statement.bindText(8, _tmpAddress.city)
        } else {
          statement.bindNull(7)
          statement.bindNull(8)
        }
      }
    }
    this.__insertAdapterOfUser_1 = object : EntityInsertAdapter<User>() {
      protected override fun createQuery(): String = "INSERT OR ABORT INTO `users` (`id`,`name`,`email`,`age`,`kind`,`score`,`addr_street`,`addr_town`) VALUES (nullif(?, 0),?,?,?,?,?,?,?)"

      protected override fun bind(statement: SQLiteStatement, entity: User) {
        statement.bindLong(1, entity.id)
        statement.bindText(2, entity.name)
        val _tmpEmail: String? = entity.email
        if (_tmpEmail == null) {
          statement.bindNull(3)
        } else {
          statement.bindText(3, _tmpEmail)
        }
        statement.bindLong(4, entity.age.toLong())
        statement.bindText(5, __Kind_enumToString(entity.kind))
        val _tmpScore: Double? = entity.score
        if (_tmpScore == null) {
          statement.bindNull(6)
        } else {
          statement.bindDouble(6, _tmpScore)
        }
        val _tmpAddress: Address? = entity.address
        if (_tmpAddress != null) {
          statement.bindText(7, _tmpAddress.street)
          statement.bindText(8, _tmpAddress.city)
        } else {
          statement.bindNull(7)
          statement.bindNull(8)
        }
      }
    }
    this.__deleteAdapterOfUser = object : EntityDeleteOrUpdateAdapter<User>() {
      protected override fun createQuery(): String = "DELETE FROM `users` WHERE `id` = ?"

      protected override fun bind(statement: SQLiteStatement, entity: User) {
        statement.bindLong(1, entity.id)
      }
    }
    this.__updateAdapterOfUser = object : EntityDeleteOrUpdateAdapter<User>() {
      protected override fun createQuery(): String = "UPDATE OR ABORT `users` SET `id` = ?,`name` = ?,`email` = ?,`age` = ?,`kind` = ?,`score` = ?,`addr_street` = ?,`addr_town` = ? WHERE `id` = ?"

      protected override fun bind(statement: SQLiteStatement, entity: User) {
        statement.bindLong(1, entity.id)
        statement.bindText(2, entity.name)
        val _tmpEmail: String? = entity.email
        if (_tmpEmail == null) {
          statement.bindNull(3)
        } else {
          statement.bindText(3, _tmpEmail)
        }
        statement.bindLong(4, entity.age.toLong())
        statement.bindText(5, __Kind_enumToString(entity.kind))
        val _tmpScore: Double? = entity.score
        if (_tmpScore == null) {
          statement.bindNull(6)
        } else {
          statement.bindDouble(6, _tmpScore)
        }
        val _tmpAddress: Address? = entity.address
        if (_tmpAddress != null) {
          statement.bindText(7, _tmpAddress.street)
          statement.bindText(8, _tmpAddress.city)
        } else {
          statement.bindNull(7)
          statement.bindNull(8)
        }
        statement.bindLong(9, entity.id)
      }
    }
  }

  public override suspend fun insert(user: User): Long = performSuspending(__db, false, true) { _connection ->
    val _result: Long = __insertAdapterOfUser.insertAndReturnId(_connection, user)
    _result
  }

  public override suspend fun insertAll(vararg users: User): List<Long> = performSuspending(__db, false, true) { _connection ->
    val _result: List<Long> = __insertAdapterOfUser_1.insertAndReturnIdsList(_connection, users)
    _result
  }

  public override suspend fun delete(user: User): Unit = performSuspending(__db, false, true) { _connection ->
    __deleteAdapterOfUser.handle(_connection, user)
  }

  public override suspend fun update(user: User): Int = performSuspending(__db, false, true) { _connection ->
    var _result: Int = 0
    _result += __updateAdapterOfUser.handle(_connection, user)
    _result
  }

  public override suspend fun byId(id: Long): User? {
    val _sql: String = "SELECT * FROM users WHERE id = ?"
    return performSuspending(__db, true, false) { _connection ->
      val _stmt: SQLiteStatement = _connection.prepare(_sql)
      try {
        var _argIndex: Int = 1
        _stmt.bindLong(_argIndex, id)
        val _columnIndexOfId: Int = getColumnIndexOrThrow(_stmt, "id")
        val _columnIndexOfName: Int = getColumnIndexOrThrow(_stmt, "name")
        val _columnIndexOfEmail: Int = getColumnIndexOrThrow(_stmt, "email")
        val _columnIndexOfAge: Int = getColumnIndexOrThrow(_stmt, "age")
        val _columnIndexOfKind: Int = getColumnIndexOrThrow(_stmt, "kind")
        val _columnIndexOfScore: Int = getColumnIndexOrThrow(_stmt, "score")
        val _columnIndexOfStreet: Int = getColumnIndexOrThrow(_stmt, "addr_street")
        val _columnIndexOfCity: Int = getColumnIndexOrThrow(_stmt, "addr_town")
        val _result: User?
        if (_stmt.step()) {
          val _tmpId: Long
          _tmpId = _stmt.getLong(_columnIndexOfId)
          val _tmpName: String
          _tmpName = _stmt.getText(_columnIndexOfName)
          val _tmpEmail: String?
          if (_stmt.isNull(_columnIndexOfEmail)) {
            _tmpEmail = null
          } else {
            _tmpEmail = _stmt.getText(_columnIndexOfEmail)
          }
          val _tmpAge: Int
          _tmpAge = _stmt.getLong(_columnIndexOfAge).toInt()
          val _tmpKind: Kind
          _tmpKind = __Kind_stringToEnum(_stmt.getText(_columnIndexOfKind))
          val _tmpScore: Double?
          if (_stmt.isNull(_columnIndexOfScore)) {
            _tmpScore = null
          } else {
            _tmpScore = _stmt.getDouble(_columnIndexOfScore)
          }
          val _tmpAddress: Address?
          if (!(_stmt.isNull(_columnIndexOfStreet) && _stmt.isNull(_columnIndexOfCity))) {
            val _tmpStreet: String
            _tmpStreet = _stmt.getText(_columnIndexOfStreet)
            val _tmpCity: String
            _tmpCity = _stmt.getText(_columnIndexOfCity)
            _tmpAddress = Address(_tmpStreet,_tmpCity)
          } else {
            _tmpAddress = null
          }
          _result = User(_tmpId,_tmpName,_tmpEmail,_tmpAge,_tmpKind,_tmpScore,_tmpAddress)
        } else {
          _result = null
        }
        _result
      } finally {
        _stmt.close()
      }
    }
  }

  public override fun olderThan(min: Int): Flow<List<User>> {
    val _sql: String = "SELECT * FROM users WHERE age > ? ORDER BY name"
    return createFlow(__db, false, arrayOf("users")) { _connection ->
      val _stmt: SQLiteStatement = _connection.prepare(_sql)
      try {
        var _argIndex: Int = 1
        _stmt.bindLong(_argIndex, min.toLong())
        val _columnIndexOfId: Int = getColumnIndexOrThrow(_stmt, "id")
        val _columnIndexOfName: Int = getColumnIndexOrThrow(_stmt, "name")
        val _columnIndexOfEmail: Int = getColumnIndexOrThrow(_stmt, "email")
        val _columnIndexOfAge: Int = getColumnIndexOrThrow(_stmt, "age")
        val _columnIndexOfKind: Int = getColumnIndexOrThrow(_stmt, "kind")
        val _columnIndexOfScore: Int = getColumnIndexOrThrow(_stmt, "score")
        val _columnIndexOfStreet: Int = getColumnIndexOrThrow(_stmt, "addr_street")
        val _columnIndexOfCity: Int = getColumnIndexOrThrow(_stmt, "addr_town")
        val _result: MutableList<User> = mutableListOf()
        while (_stmt.step()) {
          val _item: User
          val _tmpId: Long
          _tmpId = _stmt.getLong(_columnIndexOfId)
          val _tmpName: String
          _tmpName = _stmt.getText(_columnIndexOfName)
          val _tmpEmail: String?
          if (_stmt.isNull(_columnIndexOfEmail)) {
            _tmpEmail = null
          } else {
            _tmpEmail = _stmt.getText(_columnIndexOfEmail)
          }
          val _tmpAge: Int
          _tmpAge = _stmt.getLong(_columnIndexOfAge).toInt()
          val _tmpKind: Kind
          _tmpKind = __Kind_stringToEnum(_stmt.getText(_columnIndexOfKind))
          val _tmpScore: Double?
          if (_stmt.isNull(_columnIndexOfScore)) {
            _tmpScore = null
          } else {
            _tmpScore = _stmt.getDouble(_columnIndexOfScore)
          }
          val _tmpAddress: Address?
          if (!(_stmt.isNull(_columnIndexOfStreet) && _stmt.isNull(_columnIndexOfCity))) {
            val _tmpStreet: String
            _tmpStreet = _stmt.getText(_columnIndexOfStreet)
            val _tmpCity: String
            _tmpCity = _stmt.getText(_columnIndexOfCity)
            _tmpAddress = Address(_tmpStreet,_tmpCity)
          } else {
            _tmpAddress = null
          }
          _item = User(_tmpId,_tmpName,_tmpEmail,_tmpAge,_tmpKind,_tmpScore,_tmpAddress)
          _result.add(_item)
        }
        _result
      } finally {
        _stmt.close()
      }
    }
  }

  public override suspend fun names(ids: List<Long>): List<NameAndAge> {
    val _stringBuilder: StringBuilder = StringBuilder()
    _stringBuilder.append("SELECT name, age FROM users WHERE id IN (")
    val _inputSize: Int = ids.size
    appendPlaceholders(_stringBuilder, _inputSize)
    _stringBuilder.append(")")
    val _sql: String = _stringBuilder.toString()
    return performSuspending(__db, true, false) { _connection ->
      val _stmt: SQLiteStatement = _connection.prepare(_sql)
      try {
        var _argIndex: Int = 1
        for (_item: Long in ids) {
          _stmt.bindLong(_argIndex, _item)
          _argIndex++
        }
        val _columnIndexOfName: Int = 0
        val _columnIndexOfAge: Int = 1
        val _result: MutableList<NameAndAge> = mutableListOf()
        while (_stmt.step()) {
          val _item_1: NameAndAge
          val _tmpName: String
          _tmpName = _stmt.getText(_columnIndexOfName)
          val _tmpAge: Int
          _tmpAge = _stmt.getLong(_columnIndexOfAge).toInt()
          _item_1 = NameAndAge(_tmpName,_tmpAge)
          _result.add(_item_1)
        }
        _result
      } finally {
        _stmt.close()
      }
    }
  }

  public override suspend fun withNotes(): List<UserWithNotes> {
    val _sql: String = "SELECT * FROM users"
    return performSuspending(__db, true, true) { _connection ->
      val _stmt: SQLiteStatement = _connection.prepare(_sql)
      try {
        val _columnIndexOfId: Int = getColumnIndexOrThrow(_stmt, "id")
        val _columnIndexOfName: Int = getColumnIndexOrThrow(_stmt, "name")
        val _columnIndexOfEmail: Int = getColumnIndexOrThrow(_stmt, "email")
        val _columnIndexOfAge: Int = getColumnIndexOrThrow(_stmt, "age")
        val _columnIndexOfKind: Int = getColumnIndexOrThrow(_stmt, "kind")
        val _columnIndexOfScore: Int = getColumnIndexOrThrow(_stmt, "score")
        val _columnIndexOfStreet: Int = getColumnIndexOrThrow(_stmt, "addr_street")
        val _columnIndexOfCity: Int = getColumnIndexOrThrow(_stmt, "addr_town")
        val _collectionNotes: LongSparseArray<MutableList<Note>> = LongSparseArray<MutableList<Note>>()
        while (_stmt.step()) {
          val _tmpKey: Long
          _tmpKey = _stmt.getLong(_columnIndexOfId)
          if (!_collectionNotes.containsKey(_tmpKey)) {
            _collectionNotes.put(_tmpKey, mutableListOf())
          }
        }
        _stmt.reset()
        __fetchRelationshipNoteAsrNote(_connection, _collectionNotes)
        val _result: MutableList<UserWithNotes> = mutableListOf()
        while (_stmt.step()) {
          val _item: UserWithNotes
          val _tmpUser: User
          val _tmpId: Long
          _tmpId = _stmt.getLong(_columnIndexOfId)
          val _tmpName: String
          _tmpName = _stmt.getText(_columnIndexOfName)
          val _tmpEmail: String?
          if (_stmt.isNull(_columnIndexOfEmail)) {
            _tmpEmail = null
          } else {
            _tmpEmail = _stmt.getText(_columnIndexOfEmail)
          }
          val _tmpAge: Int
          _tmpAge = _stmt.getLong(_columnIndexOfAge).toInt()
          val _tmpKind: Kind
          _tmpKind = __Kind_stringToEnum(_stmt.getText(_columnIndexOfKind))
          val _tmpScore: Double?
          if (_stmt.isNull(_columnIndexOfScore)) {
            _tmpScore = null
          } else {
            _tmpScore = _stmt.getDouble(_columnIndexOfScore)
          }
          val _tmpAddress: Address?
          if (!(_stmt.isNull(_columnIndexOfStreet) && _stmt.isNull(_columnIndexOfCity))) {
            val _tmpStreet: String
            _tmpStreet = _stmt.getText(_columnIndexOfStreet)
            val _tmpCity: String
            _tmpCity = _stmt.getText(_columnIndexOfCity)
            _tmpAddress = Address(_tmpStreet,_tmpCity)
          } else {
            _tmpAddress = null
          }
          _tmpUser = User(_tmpId,_tmpName,_tmpEmail,_tmpAge,_tmpKind,_tmpScore,_tmpAddress)
          val _tmpNotesCollection: MutableList<Note>
          val _tmpKey_1: Long
          _tmpKey_1 = _stmt.getLong(_columnIndexOfId)
          _tmpNotesCollection = checkNotNull(_collectionNotes.get(_tmpKey_1))
          _item = UserWithNotes(_tmpUser,_tmpNotesCollection)
          _result.add(_item)
        }
        _result
      } finally {
        _stmt.close()
      }
    }
  }

  public override suspend fun count(kind: Kind): Int {
    val _sql: String = "SELECT count(*) FROM users WHERE kind = ?"
    return performSuspending(__db, true, false) { _connection ->
      val _stmt: SQLiteStatement = _connection.prepare(_sql)
      try {
        var _argIndex: Int = 1
        _stmt.bindText(_argIndex, __Kind_enumToString(kind))
        val _result: Int
        if (_stmt.step()) {
          val _tmp: Int
          _tmp = _stmt.getLong(0).toInt()
          _result = _tmp
        } else {
          _result = 0
        }
        _result
      } finally {
        _stmt.close()
      }
    }
  }

  public override suspend fun clear() {
    val _sql: String = "DELETE FROM users"
    return performSuspending(__db, false, true) { _connection ->
      val _stmt: SQLiteStatement = _connection.prepare(_sql)
      try {
        _stmt.step()
      } finally {
        _stmt.close()
      }
    }
  }

  private fun __Kind_enumToString(_value: Kind): String = when (_value) {
    Kind.PERSON -> "PERSON"
    Kind.BOT -> "BOT"
  }

  private fun __Kind_stringToEnum(_value: String): Kind = when (_value) {
    "PERSON" -> Kind.PERSON
    "BOT" -> Kind.BOT
    else -> throw IllegalArgumentException("Can't convert value to enum, unknown value: " + _value)
  }

  private fun __fetchRelationshipNoteAsrNote(_connection: SQLiteConnection, _map: LongSparseArray<MutableList<Note>>) {
    if (_map.isEmpty()) {
      return
    }
    if (_map.size() > 999) {
      recursiveFetchLongSparseArray(_map, true) { _tmpMap ->
        __fetchRelationshipNoteAsrNote(_connection, _tmpMap)
      }
      return
    }
    val _stringBuilder: StringBuilder = StringBuilder()
    _stringBuilder.append("SELECT `id`,`ownerId`,`text`,`created`,`tags` FROM `Note` WHERE `ownerId` IN (")
    val _inputSize: Int = _map.size()
    appendPlaceholders(_stringBuilder, _inputSize)
    _stringBuilder.append(")")
    val _sql: String = _stringBuilder.toString()
    val _stmt: SQLiteStatement = _connection.prepare(_sql)
    var _argIndex: Int = 1
    for (i in 0 until _map.size()) {
      val _item: Long = _map.keyAt(i)
      _stmt.bindLong(_argIndex, _item)
      _argIndex++
    }
    try {
      val _itemKeyIndex: Int = getColumnIndex(_stmt, "ownerId")
      if (_itemKeyIndex == -1) {
        return
      }
      val _columnIndexOfId: Int = 0
      val _columnIndexOfOwnerId: Int = 1
      val _columnIndexOfText: Int = 2
      val _columnIndexOfCreated: Int = 3
      val _columnIndexOfTags: Int = 4
      while (_stmt.step()) {
        val _tmpKey: Long
        _tmpKey = _stmt.getLong(_itemKeyIndex)
        val _tmpRelation: MutableList<Note>? = _map.get(_tmpKey)
        if (_tmpRelation != null) {
          val _item_1: Note
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
          _item_1 = Note(_tmpId,_tmpOwnerId,_tmpText,_tmpCreated,_tmpTags)
          _tmpRelation.add(_item_1)
        }
      }
    } finally {
      _stmt.close()
    }
  }

  public companion object {
    public fun getRequiredConverters(): List<KClass<*>> = emptyList()
  }
}
