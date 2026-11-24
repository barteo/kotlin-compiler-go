package lib

import dagger.Module
import dagger.Provides
import dagger.multibindings.ClassKey
import dagger.multibindings.IntoMap
import dagger.multibindings.Multibinds
import javax.inject.Named

interface Screen

class Home : Screen

class Settings : Screen

class About : Screen

// Entries keyed by Dagger's ClassKey, a Java annotation whose value is a
// Class, and by Metro's, whose value is a KClass.
@Module
object DaggerScreens {
    @Provides @IntoMap @ClassKey(Home::class) fun home(): Screen = Home()
    @Provides @IntoMap @ClassKey(Settings::class) fun settings(): Screen = Settings()
}

@Module
interface Declared {
    @Multibinds @Named("none") fun none(): Map<Class<*>, Screen>
}

@dev.zacsweers.metro.BindingContainer
object MetroScreens {
    @dev.zacsweers.metro.Provides @dev.zacsweers.metro.IntoMap @dev.zacsweers.metro.ClassKey(About::class)
    fun about(): Screen = About()
}
