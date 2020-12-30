// +build windows darwin linux,arm

package analyze

// GetDevicesInfo returns usage info about mounted devices (by calling Statfs syscall)
func GetDevicesInfo() []*Device {
	return nil
}
