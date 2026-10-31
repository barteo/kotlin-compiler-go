// The classes that kc's public inline functions (app/) create, which
// kotlinc's inliner copies into another module (use/): it runs with
// assertions, which check that each is marked part of the public ABI.
package lib

fun greet(name: String): String = "hello $name"
