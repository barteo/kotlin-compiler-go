package amazon.lastmile.inject

import k.Fake
import k.FakeObject
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = FakeObject::class)
public interface KFakeObject {
  @Provides
  public fun provideFakeObjectFake(fakeObject: FakeObject): Fake = fakeObject
}
