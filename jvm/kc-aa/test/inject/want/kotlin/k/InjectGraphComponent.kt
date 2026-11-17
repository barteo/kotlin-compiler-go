package k

import kotlin.Any
import kotlin.Int
import kotlin.Lazy
import kotlin.String
import kotlin.collections.Map
import kotlin.collections.Set
import kotlin.reflect.KClass

public fun KClass<GraphComponent>.create(token: Token): GraphComponent = InjectGraphComponent(token)

public class InjectGraphComponent(
  token: Token,
) : GraphComponent(token) {
  override val counter: Counter
    get() = Counter(
      clock = Clock()
    )

  override val requests: (String, Int) -> Request
    get() = { arg0, arg1 ->
      Request(
        path = arg0,
        retries = arg1,
        clock = Clock()
      )
    }

  override val clocks: Producer<Clock>
    get() = {
      Clock()
    }

  override val lazyClock: Lazy<Clock>
    get() = lazy {
      Clock()
    }

  override val greet: greet
    get() = { arg0 ->
      greet(
        clock = Clock(),
        times = arg0
      )
    }

  override val named: String
    get() = namedA()

  override val handlers: Map<String, Producer<Any?>>
    get() = mapOf(
      handler(
        f = clocks
      )
    )

  override val tags: Set<Greeting>
    get() = setOf(
      tag()
    )
}
