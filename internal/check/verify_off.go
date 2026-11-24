//go:build !kcverify

package check

// verifyFilter makes every lookup that the class path's name filter rules
// out run in full as well, and panic if that finds a class: build with
// -tags kcverify.
const verifyFilter = false
