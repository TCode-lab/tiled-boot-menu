package ui

/*
#cgo pkg-config: fontconfig
#include <fontconfig/fontconfig.h>
#include <stdlib.h>

static int boot_add_font(const char *path) {
	return FcConfigAppFontAddFile(FcConfigGetCurrent(), (const FcChar8 *)path);
}
*/
import "C"

import (
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"unsafe"
)

//go:embed assets/inter.ttf
var interTTF []byte

// registerBundledFont makes the design font visible to Pango through
// fontconfig. If the cache directory is not writable the system fallback is
// used instead.
func registerBundledFont() {
	dir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, "boot-menu", "Inter.ttf")
	if _, err := os.Stat(path); err != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			log.Printf("font: %v", err)
			return
		}
		if err := os.WriteFile(path, interTTF, 0o644); err != nil {
			log.Printf("font: %v", err)
			return
		}
	}
	cs := C.CString(path)
	defer C.free(unsafe.Pointer(cs))
	C.boot_add_font(cs)
}
