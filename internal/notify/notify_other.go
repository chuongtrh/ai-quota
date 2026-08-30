//go:build !darwin || !cgo

package notify

func RequestPermission() {}

func Send(title, body string) error {
	return nil
}
