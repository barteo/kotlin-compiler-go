// A class whose superclass inherits all the implementations of a member
// is checked there, unless another supertype has an intersection of them
// of its own, as K2 has it: IChild inherits func from ISecondary and ITop.
// A Kotlin interface with one supertype has none.

open class Parent : GrandParent(), ISecondary // ERROR because it inherits many implementations of it

class Intermediate : GrandParent(), IChild // ERROR because it inherits many implementations of it

class OtherIntermediate : Parent(), ISecondary

abstract class Child : Parent(), IChild // ERROR because it inherits many implementations of it

interface KChild : ISecondary

class KotlinChild : Parent(), KChild
