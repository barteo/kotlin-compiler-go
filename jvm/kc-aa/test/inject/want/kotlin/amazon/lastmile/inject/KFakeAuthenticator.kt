package amazon.lastmile.inject

import k.Authenticator
import k.FakeAuthenticator
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = FakeAuthenticator::class)
public interface KFakeAuthenticator {
  @Provides
  public fun provideFakeAuthenticatorAuthenticator(fakeAuthenticator: FakeAuthenticator): Authenticator = fakeAuthenticator
}
