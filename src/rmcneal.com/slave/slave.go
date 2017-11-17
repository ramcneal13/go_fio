package main

import (
	"net"
	"os"
	"rmcneal.com/support"
)

func main() {
	printer := support.PrintInit()

	var err error
	var ln net.Listener
	if ln, err = net.Listen("tcp", ":6969"); err != nil {
		printer.Send("Listen failed: %s\n", err)
		os.Exit(1)
	}
	for {
		sc := support.SlaveState{}
		if sc.SlaveConn, err = ln.Accept(); err != nil {
			printer.Send("WARNING: Accept failed: err=%s\n", err)
			continue
		}
		// printer.Send("Connection from: %s\n", sc.SlaveConn.RemoteAddr())
		go sc.SlaveExecute(printer)
	}
}
