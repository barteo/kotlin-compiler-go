package d

typealias Factory<T> = () -> T

typealias TagAlias = Tag

typealias TagAliasAlias = TagAlias

class Uses(val f: Factory<Point>, val g: Factory<Factory<String>>, val java: JavaThing)

@TagAlias
class Aliased(@TagAliasAlias val x: Int)
