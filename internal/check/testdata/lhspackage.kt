// The warnings kotlinc 2.4.21 reports of the type arguments of package
// qualifiers below, as TestWarningsLHSPackage reads them: each line ends
// with a comment naming the warnings it reports, if any.
package org.example.foo

class PackageTest<T> {
    inner class A<K>
}

val k1 = org.example<Int, String>.foo.PackageTest.A::class // TYPE_ARGUMENTS_NOT_ALLOWED_IN_PACKAGE_QUALIFIER_WARNING
val k2 = org.example.foo<Int>.PackageTest::class // error
val k3 = org<Int>.example.foo.PackageTest::class // TYPE_ARGUMENTS_NOT_ALLOWED_IN_PACKAGE_QUALIFIER_WARNING
