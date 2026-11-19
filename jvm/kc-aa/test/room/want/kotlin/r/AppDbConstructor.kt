package r

import androidx.room.RoomDatabaseConstructor

public actual object AppDbConstructor : RoomDatabaseConstructor<AppDb> {
  actual override fun initialize(): AppDb = r.AppDb_Impl()
}
