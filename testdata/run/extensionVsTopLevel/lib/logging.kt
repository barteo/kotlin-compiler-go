package lib

object Logging {
    enum class Priority { DEBUG, INFO }
}

inline fun Any.log(priority: Logging.Priority = Logging.Priority.DEBUG, metaData: String? = null, message: () -> String) =
    println("ext $priority $metaData ${message()}")

inline fun log(tag: String, priority: Logging.Priority = Logging.Priority.DEBUG, metaData: String? = null, message: () -> String) =
    println("top $tag $priority $metaData ${message()}")
