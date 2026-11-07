package r

import androidx.room.InvalidationTracker
import androidx.room.RoomOpenDelegate
import androidx.room.migration.AutoMigrationSpec
import androidx.room.migration.Migration
import androidx.room.util.TableInfo
import androidx.room.util.TableInfo.Companion.read
import androidx.room.util.dropFtsSyncTriggers
import androidx.sqlite.SQLiteConnection
import androidx.sqlite.execSQL
import javax.`annotation`.processing.Generated
import kotlin.Lazy
import kotlin.String
import kotlin.Suppress
import kotlin.collections.List
import kotlin.collections.Map
import kotlin.collections.MutableList
import kotlin.collections.MutableMap
import kotlin.collections.MutableSet
import kotlin.collections.Set
import kotlin.collections.mutableListOf
import kotlin.collections.mutableMapOf
import kotlin.collections.mutableSetOf
import kotlin.reflect.KClass

@Generated(value = ["androidx.room.RoomProcessor"])
@Suppress(names = ["UNCHECKED_CAST", "DEPRECATION", "REDUNDANT_PROJECTION", "REMOVAL"])
public class AppDb_Impl : AppDb() {
  private val _userDao: Lazy<UserDao> = lazy {
    UserDao_Impl(this)
  }

  private val _noteDao: Lazy<NoteDao> = lazy {
    NoteDao_Impl(this)
  }

  protected override fun createOpenDelegate(): RoomOpenDelegate {
    val _openDelegate: RoomOpenDelegate = object : RoomOpenDelegate(1, "480ea041ebcabcccafd62deba8accc14", "f76e4e3609f04e08e805af70dbb735f7") {
      public override fun createAllTables(connection: SQLiteConnection) {
        connection.execSQL("CREATE TABLE IF NOT EXISTS `users` (`id` INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, `name` TEXT NOT NULL, `email` TEXT, `age` INTEGER NOT NULL, `kind` TEXT NOT NULL, `score` REAL, `addr_street` TEXT, `addr_town` TEXT)")
        connection.execSQL("CREATE UNIQUE INDEX IF NOT EXISTS `index_users_email` ON `users` (`email`)")
        connection.execSQL("CREATE TABLE IF NOT EXISTS `Note` (`id` TEXT NOT NULL, `ownerId` INTEGER NOT NULL, `text` TEXT NOT NULL, `created` INTEGER, `tags` TEXT NOT NULL, PRIMARY KEY(`id`), FOREIGN KEY(`ownerId`) REFERENCES `users`(`id`) ON UPDATE NO ACTION ON DELETE CASCADE )")
        connection.execSQL("CREATE INDEX IF NOT EXISTS `index_Note_ownerId` ON `Note` (`ownerId`)")
        connection.execSQL("CREATE TABLE IF NOT EXISTS room_master_table (id INTEGER PRIMARY KEY,identity_hash TEXT)")
        connection.execSQL("INSERT OR REPLACE INTO room_master_table (id,identity_hash) VALUES(42, '480ea041ebcabcccafd62deba8accc14')")
      }

      public override fun dropAllTables(connection: SQLiteConnection) {
        connection.execSQL("DROP TABLE IF EXISTS `users`")
        connection.execSQL("DROP TABLE IF EXISTS `Note`")
      }

      public override fun onCreate(connection: SQLiteConnection) {
      }

      public override fun onOpen(connection: SQLiteConnection) {
        connection.execSQL("PRAGMA foreign_keys = ON")
        internalInitInvalidationTracker(connection)
      }

      public override fun onPreMigrate(connection: SQLiteConnection) {
        dropFtsSyncTriggers(connection)
      }

      public override fun onPostMigrate(connection: SQLiteConnection) {
      }

      public override fun onValidateSchema(connection: SQLiteConnection): RoomOpenDelegate.ValidationResult {
        val _columnsUsers: MutableMap<String, TableInfo.Column> = mutableMapOf()
        _columnsUsers.put("id", TableInfo.Column("id", "INTEGER", true, 1, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsUsers.put("name", TableInfo.Column("name", "TEXT", true, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsUsers.put("email", TableInfo.Column("email", "TEXT", false, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsUsers.put("age", TableInfo.Column("age", "INTEGER", true, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsUsers.put("kind", TableInfo.Column("kind", "TEXT", true, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsUsers.put("score", TableInfo.Column("score", "REAL", false, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsUsers.put("addr_street", TableInfo.Column("addr_street", "TEXT", false, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsUsers.put("addr_town", TableInfo.Column("addr_town", "TEXT", false, 0, null, TableInfo.CREATED_FROM_ENTITY))
        val _foreignKeysUsers: MutableSet<TableInfo.ForeignKey> = mutableSetOf()
        val _indicesUsers: MutableSet<TableInfo.Index> = mutableSetOf()
        _indicesUsers.add(TableInfo.Index("index_users_email", true, listOf("email"), listOf("ASC")))
        val _infoUsers: TableInfo = TableInfo("users", _columnsUsers, _foreignKeysUsers, _indicesUsers)
        val _existingUsers: TableInfo = read(connection, "users")
        if (!_infoUsers.equals(_existingUsers)) {
          return RoomOpenDelegate.ValidationResult(false, """
              |users(r.User).
              | Expected:
              |""".trimMargin() + _infoUsers + """
              |
              | Found:
              |""".trimMargin() + _existingUsers)
        }
        val _columnsNote: MutableMap<String, TableInfo.Column> = mutableMapOf()
        _columnsNote.put("id", TableInfo.Column("id", "TEXT", true, 1, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsNote.put("ownerId", TableInfo.Column("ownerId", "INTEGER", true, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsNote.put("text", TableInfo.Column("text", "TEXT", true, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsNote.put("created", TableInfo.Column("created", "INTEGER", false, 0, null, TableInfo.CREATED_FROM_ENTITY))
        _columnsNote.put("tags", TableInfo.Column("tags", "TEXT", true, 0, null, TableInfo.CREATED_FROM_ENTITY))
        val _foreignKeysNote: MutableSet<TableInfo.ForeignKey> = mutableSetOf()
        _foreignKeysNote.add(TableInfo.ForeignKey("users", "CASCADE", "NO ACTION", listOf("ownerId"), listOf("id")))
        val _indicesNote: MutableSet<TableInfo.Index> = mutableSetOf()
        _indicesNote.add(TableInfo.Index("index_Note_ownerId", false, listOf("ownerId"), listOf("ASC")))
        val _infoNote: TableInfo = TableInfo("Note", _columnsNote, _foreignKeysNote, _indicesNote)
        val _existingNote: TableInfo = read(connection, "Note")
        if (!_infoNote.equals(_existingNote)) {
          return RoomOpenDelegate.ValidationResult(false, """
              |Note(r.Note).
              | Expected:
              |""".trimMargin() + _infoNote + """
              |
              | Found:
              |""".trimMargin() + _existingNote)
        }
        return RoomOpenDelegate.ValidationResult(true, null)
      }
    }
    return _openDelegate
  }

  protected override fun createInvalidationTracker(): InvalidationTracker {
    val _shadowTablesMap: MutableMap<String, String> = mutableMapOf()
    val _viewTables: MutableMap<String, Set<String>> = mutableMapOf()
    return InvalidationTracker(this, _shadowTablesMap, _viewTables, "users", "Note")
  }

  protected override fun getRequiredTypeConverterClasses(): Map<KClass<*>, List<KClass<*>>> {
    val _typeConvertersMap: MutableMap<KClass<*>, List<KClass<*>>> = mutableMapOf()
    _typeConvertersMap.put(UserDao::class, UserDao_Impl.getRequiredConverters())
    _typeConvertersMap.put(NoteDao::class, NoteDao_Impl.getRequiredConverters())
    return _typeConvertersMap
  }

  public override fun getRequiredAutoMigrationSpecClasses(): Set<KClass<out AutoMigrationSpec>> {
    val _autoMigrationSpecsSet: MutableSet<KClass<out AutoMigrationSpec>> = mutableSetOf()
    return _autoMigrationSpecsSet
  }

  public override fun createAutoMigrations(autoMigrationSpecs: Map<KClass<out AutoMigrationSpec>, AutoMigrationSpec>): List<Migration> {
    val _autoMigrations: MutableList<Migration> = mutableListOf()
    return _autoMigrations
  }

  public override fun users(): UserDao = _userDao.value

  public override fun notes(): NoteDao = _noteDao.value
}
