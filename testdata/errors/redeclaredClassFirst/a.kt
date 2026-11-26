// A redeclared class or typealias names the first declaration, as with
// kotlinc: the other files' uses of a private one can't see it.

private class C { fun a() = 1 } // ERROR redeclaration
fun fa() = C().a()

sealed interface SE // ERROR redeclaration: SE
abstract class SE : SN // ERROR redeclaration: class SE
interface SN : SE
class SES : SE() // ERROR this type has no constructor
