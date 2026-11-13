package amazon.lastmile.inject

import k.Authenticator
import k.RealAuthenticator
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = RealAuthenticator::class)
public interface KRealAuthenticator {
  @Provides
  public fun provideRealAuthenticatorAuthenticator(realAuthenticator: RealAuthenticator): Authenticator = realAuthenticator
}
