package org.jetbrains.annotations;

import java.lang.annotation.ElementType;
import java.lang.annotation.Target;

// As the annotations' jar declares it, which the test doesn't have.
@Target({ElementType.METHOD, ElementType.FIELD, ElementType.PARAMETER, ElementType.LOCAL_VARIABLE, ElementType.TYPE_USE})
public @interface NotNull {}
