package support

import (
	"fmt"
)

const (
	_        = iota
	PrintStr = iota + 1
	PrintLn
	PrintExit
	PrintGroupStart
	PrintGroupStr
	PrintGroupEnd
)

type PrintOp struct {
	OpType int
	OpStr  string
	OpChan chan int
}

type Printer struct {
	incoming chan PrintOp
	group	bool
}

func PrintInit() *Printer {
	p := &Printer{}
	p.incoming = make(chan PrintOp)
	go p.printWorker()
	return p
}

func (p *Printer) printWorker() {
	for {
		op := <-p.incoming
		switch op.OpType {
		case PrintStr:
			if p.group {
				// Use an anonymous function to push the message back on the queue. If not done
				// the channel can be full causing the print thread to stall and not remove other
				// items from the queue to clear it.
				go func() {
					p.incoming <- op
				}()
			} else {
				fmt.Printf(op.OpStr)
			}
		case PrintLn:
			if p.group {
				p.incoming <- op
			} else {
				fmt.Println()
			}
		case PrintExit:
			op.OpChan <- 1
			break
		case PrintGroupStart:
			p.group = true
		case PrintGroupEnd:
			p.group = false
		case PrintGroupStr:
			fmt.Printf(op.OpStr)
		}
	}
}

func (p *Printer) Send(format string, a ...interface{}) {
	p.incoming <- PrintOp{OpType: PrintStr, OpStr: fmt.Sprintf(format, a...)}
}

func (p *Printer) Exit() {
	c := make(chan int)
	p.incoming <- PrintOp{OpType: PrintExit, OpChan: c}
	<-c
}
