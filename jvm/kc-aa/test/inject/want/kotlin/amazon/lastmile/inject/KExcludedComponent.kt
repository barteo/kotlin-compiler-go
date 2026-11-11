package amazon.lastmile.inject

import k.ExcludedComponent
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = ExcludedComponent::class)
public interface KExcludedComponent : ExcludedComponent
