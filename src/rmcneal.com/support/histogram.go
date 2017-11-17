package support

import (
	"fmt"
	//	"math"
	"math"
	"os"
	"strings"
	"time"
)

type DistroGraph struct {
	title   string
	printer *Printer
	/* ---- Bins must be capitalized so that json.Marshal can encode the data ---- */
	Bins []int64
	/* ---- The rest is used only for linear histograms ---- */
	linear   bool
	lower    time.Duration
	upper    time.Duration
	interval time.Duration
}

func DistroInit(printer *Printer, title string) *DistroGraph {
	d := &DistroGraph{}
	d.title = title
	d.printer = printer
	d.Bins = make([]int64, 64)
	d.linear = false

	return d
}

func (d *DistroGraph) CreateLinear(min time.Duration, max time.Duration, interval time.Duration) {
	d.linear = true
	d.lower = min
	d.upper = max
	d.interval = interval
	entries := (max - min) / interval
	d.Bins = make([]int64, entries)
}

func (d *DistroGraph) addBins(copy *DistroGraph) {
	if copy == nil {
		DebugLog("copy pointer is nil\n")
		return
	}
	for k, v := range copy.Bins {
		d.Bins[k] += v
	}
}

func (d *DistroGraph) Add(src *DistroGraph) {
	for k, v := range src.Bins {
		d.Bins[k] = v
	}
}

func (d *DistroGraph) Aggregate(t time.Duration) {
	idx := 0
	if d.linear {
		for i := d.lower; i < d.upper; i += d.interval {
			if t <= i {
				d.Bins[idx]++
				return
			}
			idx++
		}
		/* ---- Account for this value in the last bucket ---- */
		d.Bins[idx-1]++
	} else {
		v := int64(t)
		i := 0
		for v != 0 {
			v = v >> 1
			i++
		}
		d.Bins[i]++
	}
}

func (d *DistroGraph) Graph() {
	/* ---- Figure out column widths to start ---- */
	distroCol := 0
	countCol := 0
	count := int64(0)
	windowSize := 0
	lastIdx := 0
	firstIdx := -1
	for k, v := range d.Bins {
		if v >= count {
			count = v
		}
		/*
		 * Keep track of where values start end end. This will enable the code
		 * to only display the portion of the distribution that's relevant.
		 */
		if v > 0 {
			lastIdx = k
			if firstIdx == -1 {
				firstIdx = k
			}
		}
	}
	if count == 0 {
		return
	}
	distroCol = lastIdx

	if d.linear {
		/*
		 * Three is add here to account for the '<= ' or '>= ' which is displayed on the first and last
		 * increment values. This allows 'scalerCol' to be correctly calculated. When displaying the
		 * actual increment values three will need to be subtracted.
		 */
		distroCol = len(fmt.Sprintf("%s", d.lower+(time.Duration(lastIdx)*d.interval))) + 3
	} else {
		/* ---- Need to TrimSpace because %.0f adds a few spaces to the front of the string ---- */
		distroCol = len(strings.TrimSpace(fmt.Sprintf("%.0f", math.Pow(2, float64(lastIdx)))))
	}
	countCol = len(fmt.Sprintf("%d", count))

	if ws, err := GetWinsize(os.Stdout.Fd()); err != nil {
		windowSize = 80
	} else {
		windowSize = int(ws.Width)
	}
	scalerCol := windowSize - 2 - distroCol - countCol
	scaler := count / int64(scalerCol)

	if d.title != "" {
		if len(d.title) <= scalerCol {
			d.printer.Send("%*s%*s%s\n", distroCol, "", (scalerCol-len(d.title))/2, "", d.title)
		} else {
			d.printer.Send(d.title)
		}
	}
	d.printer.Send("%*s%s\n", distroCol, "", DashLine(scalerCol))

	for k, v := range d.Bins {
		if k >= firstIdx && k <= lastIdx {
			if d.linear {
				switch {
				case k == firstIdx:
					d.printer.Send("<= ")
				case k == lastIdx:
					d.printer.Send(">= ")
				default:
					d.printer.Send("   ")
				}
				d.printer.Send("%*s|", distroCol-3, d.lower+(time.Duration(k)*d.interval))
			} else {
				d.printer.Send("%*.0f|", distroCol, math.Pow(2, float64(k)))
			}

			for i := 0; i < scalerCol; i++ {
				if int(v/scaler) >= i {
					d.printer.Send("@")
				} else {
					d.printer.Send(" ")
				}
			}

			d.printer.Send("|%*d\n", countCol, v)
		}
	}
}
