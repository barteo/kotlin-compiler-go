// A supertype written through a nested class of the Java sources depends
// on its outer class's supertypes, which here lead back.
interface ExceptionTracker : LockBasedStorageManager.ExceptionHandlingStrategy // ERROR cycle in the inheritance hierarchy

interface StorageManager : ExceptionTracker { // ERROR cycle in the inheritance hierarchy
    fun foo()
}

interface Fine : Holder.Nested
