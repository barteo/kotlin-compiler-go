package lib

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject
import dagger.multibindings.IntoMap
import dagger.multibindings.IntoSet
import dagger.multibindings.StringKey
import javax.inject.Inject

interface Greeter {
    fun greet(): String
}

class EnglishGreeter @Inject constructor() : Greeter {
    override fun greet() = "hello"
}

data class Plugin(val name: String)

// Dagger modules of the class path: an object, an interface of @Binds and
// an abstract class whose companion object provides.
@Module
object ToolsModule {
    @Provides fun tool(): String = "hammer"
    @Provides @IntoSet fun plugin(): Plugin = Plugin("a")
}

@Module
interface GreeterModule {
    @Binds fun greeter(g: EnglishGreeter): Greeter
    @Binds @IntoMap @StringKey("en") fun english(g: EnglishGreeter): Greeter
}

@Module
abstract class CompanionModule {
    companion object {
        @Provides fun count(): Int = 3
    }
}

// Assisted parameters of the same type, told apart by Dagger's identifiers.
class Range @AssistedInject constructor(@Assisted("a") val from: Int, @Assisted("b") val to: Int, val greeter: Greeter) {
    override fun toString() = "$from..$to ${greeter.greet()}"
}
