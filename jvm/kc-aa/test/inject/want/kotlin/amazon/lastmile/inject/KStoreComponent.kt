package amazon.lastmile.inject

import k.StoreComponent
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = StoreComponent::class)
public interface KStoreComponent : StoreComponent
