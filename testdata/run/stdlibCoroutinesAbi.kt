// kotlinc: -jvm-default=no-compatibility
@file:Suppress("INVISIBLE_REFERENCE", "INVISIBLE_MEMBER")
// The JVM members of kotlinx.coroutines' shapes that kotlinc writes,
// read by reflection: names, descriptors and flags.

import java.lang.reflect.Modifier
import kotlin.coroutines.*
import kotlin.internal.InlineOnly

// An interface with JVM default methods that inherits bodies from one
// without them, CoroutineContext.Element: its DefaultImpls calls theirs.
interface Job : CoroutineContext.Element {
  fun name(): String = "job"
}

interface ChildJob : Job

// The superclass has the methods that call the bodies; its subclass,
// which lists the interface again, doesn't.
abstract class JobSupport : Job {
  override val key: CoroutineContext.Key<*> get() = Key
  object Key : CoroutineContext.Key<JobSupport>
}

class Coroutine : JobSupport(), Job, Continuation<Int> {
  override val context: CoroutineContext get() = EmptyCoroutineContext
  override fun resumeWith(result: Result<Int>) {}
}

interface Iterator {
  suspend fun hasNext(): Boolean

  @Deprecated("", level = DeprecationLevel.HIDDEN)
  @Suppress("INAPPLICABLE_JVM_NAME")
  @JvmName("next")
  suspend fun next0(): Int {
    if (!hasNext()) throw NoSuchElementException()
    return 1
  }

  suspend fun delay(n: Long): Long = n
}

class Locks {
  @Synchronized fun locked() = 1
  @Strictfp fun strict(x: Double) = x * 2
}

@InlineOnly
inline fun inlineOnly(x: Int = 1): Int = x + 1

object Dispatchers {
  val IO = "io"
}

@Suppress("EXTENSION_SHADOWED_BY_MEMBER")
@Deprecated("", level = DeprecationLevel.HIDDEN)
val Dispatchers.IO: String get() = Dispatchers.IO

class WeakMap<K : Any, V : Any> : java.util.AbstractMap<K, V>() {
  override val size: Int get() = 0
  override fun get(key: K): V? = null
  override val keys: MutableSet<K> get() = HashSet()
  override val entries: MutableSet<MutableMap.MutableEntry<K, V>> get() = HashSet()
}

abstract class Operator(@JvmField protected val flow: List<Int>)

class ListOperator : Operator(listOf(1)) {
  fun first() = flow[0]
}

fun flags(m: Int, synthetic: Boolean): String {
  var s = Modifier.toString(m)
  if (synthetic) s += " synthetic"
  return s.trim()
}

fun show(cls: Class<*>, vararg names: String) {
  val methods = cls.declaredMethods.filter { names.isEmpty() || it.name in names }
    .map { it.name + it.parameterTypes.joinToString(",", "(", ")") { p -> p.simpleName } + it.returnType.simpleName + " " + flags(it.modifiers, it.isSynthetic) }
  val fields = cls.declaredFields.filter { it.name in names }
    .map { "field " + it.name + " " + it.type.simpleName + " " + flags(it.modifiers, it.isSynthetic) }
  for (s in (fields + methods).sorted()) println(cls.simpleName + "." + s)
}

fun main() {
  val c = Coroutine()
  println(c[JobSupport.Key] === c)
  println(c.name() + " " + (c + EmptyCoroutineContext === c) + " " + Locks().locked() + " " + Locks().strict(1.5) + " " + inlineOnly())
  println(WeakMap<String, Int>().size.toString() + " " + ListOperator().first())
  show(Class.forName("Job\$DefaultImpls"))
  show(Class.forName("ChildJob\$DefaultImpls"))
  show(JobSupport::class.java, "fold", "get", "minusKey", "plus")
  show(Coroutine::class.java, "fold", "get", "minusKey", "plus")
  show(Iterator::class.java, "next", "next\$suspendImpl", "delay", "delay\$suspendImpl")
  show(Locks::class.java)
  show(Class.forName("StdlibCoroutinesAbiKt"), "inlineOnly", "inlineOnly\$default", "getIO")
  show(WeakMap::class.java, "size", "getSize", "keySet", "getKeys", "entrySet")
  show(Operator::class.java, "flow")
}
