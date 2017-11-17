package support

// Winsize stores the Heighty and Width of a terminal.
type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16 // unused
	y      uint16 // unused
}

// GetWinsize returns the size of the given tty.
func GetWinsize(fd uintptr) (*Winsize, error) {
	ws := &Winsize{Width: 80}
	return ws, nil
}
