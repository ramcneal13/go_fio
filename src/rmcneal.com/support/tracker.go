
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
	display      func()
}

func TrackingInit(printer *Printer) *tracking {
	t := &tracking{}
	t.completeChan = make(chan threadStatus, 10)
	t.printer = printer
	t.nodes = map[string]*trackingInfo{}
	t.display = emptyDisplayFunc
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

func (t *tracking) UpdateName(name string, extra string) {
	ti := t.nodes[name]
	ti.extraTag = extra
}

func (t *tracking) DisplaySet(display func()) {
	t.display = display
}

func (t *tracking) DisplayCount() {
	t.display = func() {
		cols := 80
		if win, err := GetWinsize(os.Stdout.Fd()); err == nil {
			cols = int(win.Width)
		}
		t.printer.Send("%*s\r%s ... %d\r", cols, "", t.title, t.count)
	}
}

func (t *tracking) DisplayExtra() {
	t.display = t.displayTrack
}

func emptyDisplayFunc() {}

func (t *tracking) DisplayReset() {
	t.display = emptyDisplayFunc
}

func (t *tracking) WaitForThreads() bool {
	returnState := true

	intrChans := make(chan os.Signal, 1)
	signal.Notify(intrChans, os.Interrupt, os.Kill)

	tSec := time.Tick(time.Second)
	for t.count > 0 {
		select {
		case status := <-t.completeChan:
			if !status.okay {
				// If functions returns an error need to consider everything an
				// error. This is most likely to occur because of either an I/O
				// error or the user pressed control-C during the prepare phase
				// in an attempt to stop the run early.
				returnState = false
			}
			t.removeNode(status.name)

		case <-tSec:
			t.display()

		case <-intrChans:
			for _, v := range t.nodes {
				v.stopper()
			}
		}
	}
	var cols = 80

	if win, err := GetWinsize(os.Stdout.Fd()); err == nil {
		cols = int(win.Width)
	}
	t.printer.Send("%*s\r%s ... done\n", cols-1, "", t.title)

	return returnState
}

func (t *tracking) displayTrack() {
	var cols = 80

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
}

func (t *tracking) removeNode(name string) {
	ti := t.nodes[name]
	ti.seen = false
	t.count--
}
