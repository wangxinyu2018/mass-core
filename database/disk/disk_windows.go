// +build windows

package disk

import (
	"unsafe"

	"github.com/wangxinyu2018/mass-core/logging"
	"golang.org/x/sys/windows"
)

var (
	modkernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetDiskFreeSpaceExW = modkernel32.NewProc("GetDiskFreeSpaceExW")
)

func init() {
	CheckDiskSpaceStub = CheckDiskSpaceWin
}

func CheckDiskSpaceWin(path string, additional uint64) bool {
	lpFreeBytesAvailable := int64(0)
	ret, _, err := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&lpFreeBytesAvailable)),
		uintptr(unsafe.Pointer(nil)), uintptr(unsafe.Pointer(nil)))
	if ret == 0 {
		logging.CPrint(logging.ERROR, "CheckDiskSpace error", logging.LogFormat{"err": err, "path": path})
	} else {
		logging.CPrint(logging.DEBUG, "CheckDiskSpace", logging.LogFormat{"available": lpFreeBytesAvailable, "path": path})
	}
	return lpFreeBytesAvailable >= int64(additional+MinDiskSpace)
}
