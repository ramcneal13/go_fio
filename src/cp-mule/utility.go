package main

import (
	"strings"
	"strconv"
	"fmt"
)

func blkStringToInt64(s string) (int64, bool) {
	var size int64

	if idx := strings.IndexAny(s, "kmgtKMGT"); idx != -1 {
		val, err := strconv.ParseInt(s[:idx], 10, 64)
		if err != nil {
			return 0, false
		}
		switch s[idx : idx+1] {
		case "k", "K":
			size = val * 1024
		case "m", "M":
			size = val * 1024 * 1024
		case "g", "G":
			size = val * 1024 * 1024 * 1024
		case "t", "T":
			size = val * 1024 * 1024 * 1024 * 1024
		}
	} else {
		var err error
		if size, err = strconv.ParseInt(s, 10, 64); err != nil {
			return 0, false
		}
	}
	return size, true
}

func Humanize(val int64, base int) string {
	idx := 0

	if base == 0 {
		base = 1
	}
	val *= int64(base)
	return humanizeImpl(float64(val), idx)
}

func humanizeImpl(val float64, idx int) string {
	suffix := " KMGTPE"

	switch {
	case val < 10.0:
		return fmt.Sprintf("%5.2f%c", val, suffix[idx])
	case val < 100.0:
		return fmt.Sprintf("%5.1f%c", val, suffix[idx])
	case val < 1024.0:
		return fmt.Sprintf("%5.0f%c", val, suffix[idx])
	default:
		return humanizeImpl(val/1024.0, idx+1)
	}
}

