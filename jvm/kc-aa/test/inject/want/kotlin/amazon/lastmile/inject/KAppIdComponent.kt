package amazon.lastmile.inject

import k.AppIdComponent
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = AppIdComponent::class)
public interface KAppIdComponent : AppIdComponent
