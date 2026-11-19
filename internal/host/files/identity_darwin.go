//go:build darwin && cgo

package files

/*
#include <sys/attr.h>
#include <unistd.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
static int el_volume_uuid(const char *path, unsigned char *out) {
 struct attrlist attrs = {0};
 struct { uint32_t length; unsigned char uuid[16]; } result = {0};
 attrs.bitmapcount = ATTR_BIT_MAP_COUNT;
 attrs.volattr = ATTR_VOL_INFO | ATTR_VOL_UUID;
 if (getattrlist(path, &attrs, &result, sizeof(result), 0) != 0) return errno;
 if (result.length != sizeof(result)) return EINVAL;
 memcpy(out, result.uuid, 16);
 return 0;
}
*/
import "C"
import (
	"encoding/hex"
	"syscall"
	"unsafe"
)

func nativeVolumeIdentity(path string) (string, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var id [16]byte
	code := C.el_volume_uuid(p, (*C.uchar)(unsafe.Pointer(&id[0])))
	if code != 0 {
		return "", syscall.Errno(code)
	}
	return "darwin:" + hex.EncodeToString(id[:]), nil
}
