package support

import (
	"fmt"
	"os/signal"
	"time"
)
import "os"

type threadStatus struct {
	name string
	okay bool
}

type trackingInfo struct {
	seen     bool
	extraTag string
	stopper  func()
}

type tracking struct {
	title        string
	nodes        map[string]*trackingInfo
	printer      *Printer
	count        int
	completeChan chan threadStatus
	verbose      bool
	doStats      bool
	stats        *StatsState
}

func TrackingInit(printer *Printer, stats *StatsState) *tracking {
	t := &tracking{}
	t.completeChan = make(chan threadStatus, 10)
	t.printer = printer
	t.nodes = map[string]*trackingInfo{}
	t.stats = stats
	return t
}

func (t *tracking) RunFunc(name string, f func() bool, stopper func()) {
	t.addNode(name, stopper)
	go func() {
		if f() {
			t.completeChan <- threadStatus{name: name, okay: true}
		} else {
			t.completeChan <- threadStatus{name, false}
		}
	}()
}

func (t *tracking) SetTitle(title string) {
	t.title = title
}

func (t *tracking) VerboseSet() {
	t.verbose = true
}

func (t *tracking) VerboseClear() {
	t.verbose = false
}

func (t *tracking) StatsEnable() {
	// Clear out the stats just before starting the jobs. The timer is running
	// in the stats thread which means the time spent during the prepare phase
	// would be counted against the elapsed time for these threads if we don't
	// clear the stats now.
	t.stats.Send(StatsRecord{OpType: StatClear})
	t.stats.Send(StatsRecord{OpType: StatRelDisplay})
	t.doStats = true
}

func (t *tracking) StatsDisable() {
	t.doStats = false
}

func (t *tracking) UpdateName(name string, extra string) {
	ti := t.nodes[name]
	ti.extraTag = extra
}

func (t *tracking) WaitForThreads() {
	var cols = 80

	intrChans := make(chan os.Signal, 1)
	signal.Notify(intrChans, os.Interrupt, os.Kill)

	if win, err := GetWinsize(os.Stdout.Fd()); err == nil {
		cols = int(win.Width)
	}
	tSec := time.Tick(time.Second)
	for t.count > 0 {
		select {
		case status := <-t.completeChan:
			t.removeNode(status.name)
			if t.verbose == false {
				t.printer.Send("%*s\r%s ... %d\r", cols, "", t.title, t.count)
			}
		case <-tSec:
			t.displayTrack()

		case <-intrChans:
			for _, v := range t.nodes {
				v.stopper()
			}
		}
	}
	t.printer.Send("\n")
}

func (t *tracking) displayTrack() {
	var cols = 80

	if t.doStats {
		t.printer.Send(t.stats.String())
		return
	}

	if t.verbose == false {
		return
	}
	if win, err := GetWinsize(os.Stdout.Fd()); err == nil {
		cols = int(win.Width)
	}
	t.printer.Send("%*s\r", cols-1, "")
	title := fmt.Sprintf("%s: ", t.title)
	cols -= len(title)
	t.printer.Send(title)
	for k, v := range t.nodes {
		if v.seen {
			o := fmt.Sprintf("[%s%s] ", k, v.extraTag)
			if (cols - len(o)) <= len("...") {
				t.printer.Send("...")
				break
			}
			t.printer.Send(o)
			cols -= len(o)
		}
	}
	t.printer.Send("\r")
}
func (t *tracking) addNode(name string, stopper func()) {
	t.nodes[name] = &trackingInfo{seen: true, extraTag: "", stopper: stopper}
	t.count++
	t.displayTrack()
}

func (t *tracking) removeNode(name string) {
	ti := t.nodes[name]
	ti.seen = false
	t.count--
	t.displayTrack()
}
