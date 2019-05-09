package support

import (
	"fmt"
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
}

type tracking struct {
	title        string
	nodes        map[string]*trackingInfo
	printer      *Printer
	count        int
	completeChan chan threadStatus
	verbose      bool
}

func TrackingInit(printer *Printer) *tracking {
	t := &tracking{}
	t.completeChan = make(chan threadStatus, 10)
	t.printer = printer
	t.nodes = map[string]*trackingInfo{}
	return t
}

func (t *tracking) RunFunc(name string, f func() bool) {
	t.addNode(name)
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

func (t *tracking) SetVerbose() {
	t.verbose = true
}

func (t *tracking) UpdateName(name string, extra string) {
	ti := t.nodes[name]
	ti.extraTag = extra
}

func (t *tracking) WaitForThreads() {
	var cols = 80

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
		}
	}
	t.printer.Send("\n")
}

func (t *tracking) displayTrack() {
	var cols = 80
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
func (t *tracking) addNode(name string) {
	t.nodes[name] = &trackingInfo{seen: true, extraTag: ""}
	t.count++
	t.displayTrack()
}

func (t *tracking) removeNode(name string) {
	ti := t.nodes[name]
	ti.seen = false
	t.count--
	t.displayTrack()
}
