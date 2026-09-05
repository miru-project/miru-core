//go:build android

// Package binary is compiled into libmiru_core.so by the dart build hook
// (`hook/build.dart`, `go build -buildmode=c-shared`).
package binary

/*
#include <errno.h>

// The c-shared build compiles the Go "linux" build of modernc.org/libc for
// GOOS=android (Android satisfies the "linux" build constraint), which
// expects glibc's __errno_location() to fetch the per-thread errno.
//
// Android's bionic libc does NOT provide __errno_location(); it exposes
// __errno() instead. As a result the c-shared libmiru_core.so carries an
// undefined reference to __errno_location and DynamicLibrary.open fails to
// load it:
//
//	dlopen failed: cannot locate symbol
//	"__errno_location" referenced by ".../lib/arm64/libmiru_core.so"
//
// This shim defines __errno_location() in terms of bionic's __errno(),
// satisfying the reference so the shared library loads on Android.
int* __errno_location(void) {
	return __errno();
}
*/
import "C"

// ensureErrnoShimLinked keeps the C shim symbol referenced from Go so the
// linker does not drop it from libmiru_core.so.
func ensureErrnoShimLinked() {
	_ = C.__errno_location
}
