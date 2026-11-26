package analysis

import "github.com/barteo/kotlin-compiler-go/internal/classpath"

// FindJDK returns the home directory of the JDK that kc uses, and that an
// analysis reads when Config.JDKHome is empty: JAVA_HOME if it is set,
// else the JDK of the java command on PATH.
func FindJDK() (string, error) { return classpath.FindJDK() }

// FindStdlib returns the path of the kotlin-stdlib.jar that kc finds:
// in KOTLIN_HOME/lib if it is set, else next to the kotlinc command on
// PATH. An analysis doesn't add it to Config.ClassPath by itself.
func FindStdlib() (string, error) { return classpath.FindStdlib() }
