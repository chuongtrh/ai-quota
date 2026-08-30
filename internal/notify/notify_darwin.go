//go:build darwin && cgo

package notify

/*
#cgo CFLAGS: -fblocks
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#include <stdlib.h>

void aiquota_request_notification_permission(void);
int aiquota_send_notification(const char *title, const char *body);
*/
import "C"

import (
	"errors"
	"unsafe"
)

func RequestPermission() {
	C.aiquota_request_notification_permission()
}

func Send(title, body string) error {
	cTitle := C.CString(title)
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cBody))
	if result := C.aiquota_send_notification(cTitle, cBody); result != 0 {
		return errors.New("macOS rejected the notification")
	}
	return nil
}
