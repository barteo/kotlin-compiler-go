package amazon.lastmile.inject

import k.Loader
import k.StringLoader
import kotlin.String
import me.tatarka.inject.annotations.Assisted
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = StringLoader::class)
public interface KStringLoader {
  @Provides
  public fun provideStringLoaderLoader(@Assisted path: String, realFactory: (String) -> StringLoader): Loader = realFactory(path)
}
