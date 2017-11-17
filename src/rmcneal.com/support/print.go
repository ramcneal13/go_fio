package support

import (
	"fmt"
)

const (
	_        = iota
	PrintStr = iota + 1
	PrintLn
	PrintExit
)

type PrintOp struct {
	OpType int
	OpStr  string
	OpChan chan int
}

type Printer struct {
	incoming chan PrintOp
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
			fmt.Printf(op.OpStr)
		case PrintLn:
			fmt.Println()
		case PrintExit:
			op.OpChan <- 1
			break
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
