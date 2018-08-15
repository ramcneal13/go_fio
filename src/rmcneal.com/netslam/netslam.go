package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"rmcneal.com/support"
	"sync"
	"time"
)

var asDeamon bool
var clientHost string
var requestSize int
var runTime int
var readPercent int

func init() {
	flag.BoolVar(&asDeamon, "d", false, "start as daemon")
	flag.StringVar(&clientHost, "h", "", "client host to contact")
	flag.IntVar(&requestSize, "s", 1024, "Request size")
	flag.IntVar(&runTime, "t", 60, "Run time")
	flag.IntVar(&readPercent, "r", 50, "Read percentage")
}

func main() {
	flag.Parse()

	if asDeamon {
		runDeamon()
	} else if clientHost == "" {
		fmt.Printf("Must have -h or -d minimum\n")
		os.Exit(1)
	} else {
		clientOp()
	}
}

const (
	InitialOp  = 1
	ReadOp     = 2
	WriteOp    = 3
	ExitOp     = 4
	ReadReply  = 5
	WriteReply = 6
	ExitReply  = 7
	Version    = 0x13
)

type request struct {
	Op   int
	Size int
	Vers int
}
type reply struct {
	Op   int
	Size int
}

func clientOp() {
	var wg sync.WaitGroup
	buf := make([]byte, requestSize)
	complete := time.After(time.Duration(runTime) * time.Second)

	conn, err := net.Dial("tcp", clientHost+":3600")
	if err != nil {
		fmt.Printf("Failed to connect to %s: err=%s\n", clientHost, err)
		return
	}
	enc := gob.NewEncoder(conn)
	err = enc.Encode(request{InitialOp, requestSize, Version})
	if err != nil {
		fmt.Printf("Encode err: %s\n", err)
	}

	wg.Add(1)
	defer func() {
		enc.Encode(request{ExitOp, 0, Version})
		wg.Wait()
	}()

	go clientReader(conn, requestSize, &wg)
	for {
		select {
		case <-complete:
			return
		default:
			if rand.Intn(100) > readPercent {
				enc.Encode(request{WriteOp, requestSize, Version})
				enc.Encode(&buf)
			} else {
				enc.Encode(request{ReadOp, requestSize, Version})
			}
		}
	}
}

func clientReader(conn net.Conn, size int, wg *sync.WaitGroup) {
	var lastVal, bw int64 = 0, 0
	var reply reply
	buf := make([]byte, size)
	dec := gob.NewDecoder(conn)
	tick := time.Tick(time.Second)
	tickCount := 1

	defer func() {
		fmt.Printf("Average: %s\n", support.Humanize(bw/int64(tickCount), 1))
		wg.Done()
	}()

	for {
		select {
		case <-tick:
			currentVal := bw - lastVal
			lastVal = bw
			tickCount += 1
			fmt.Printf("%s: %s\r", support.SecsToHMSstr(tickCount), support.Humanize(currentVal, 1))

		default:
			err := dec.Decode(&reply)
			if err != nil {
				fmt.Printf("Reply reader header error: %s\n", err)
				return
			}
			switch reply.Op {
			case ReadReply:
				err = dec.Decode(&buf)
				if err != nil {
					fmt.Printf("Reply error on read data: %s\n", err)
					return
				}
				bw += int64(reply.Size)
			case WriteReply:
				bw += int64(reply.Size)

			case ExitReply:
				return
			}
		}
	}
}

func runDeamon() {
	ln, err := net.Listen("tcp", ":3600")
	if err != nil {
		fmt.Printf("Listen failed: err=%s\n", err)
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Accept failed, err=%s\n", err)
			return
		}
		serverConn(conn)
		conn.Close()
	}
}

func serverConn(conn net.Conn) {
	var op request

	dec := gob.NewDecoder(conn)
	enc := gob.NewEncoder(conn)
	err := dec.Decode(&op)
	if err != nil {
		fmt.Printf("Decode error: %s\n", err)
		return
	}

	if op.Op != InitialOp {
		fmt.Print("App protocol error. First packet not an InitialOp\n")
		return
	} else if op.Vers != Version {
		fmt.Printf("Bad version: Expected %d, Got %d\n", Version, op.Vers)
		return
	}
	buf := make([]byte, op.Size)
	for {
		err := dec.Decode(&op)
		if err != nil {
			fmt.Printf("Decode op header error: %s\n", err)
			return
		}

		switch op.Op {
		case InitialOp:
			fmt.Printf("App protocol error in loop\n")
			return
		case ReadOp:
			err = enc.Encode(reply{ReadReply, op.Size})
			if err != nil {
				fmt.Printf("Encode of Read reply header: %s\n", err)
				return
			}
			err = enc.Encode(&buf)
			if err != nil {
				fmt.Printf("Encode buf error: %s\n", err)
				return
			}
		case WriteOp:
			err = dec.Decode(&buf)
			if err != nil {
				fmt.Printf("Decode buf error: %s\n", err)
				return
			}
			err = enc.Encode(reply{WriteReply, op.Size})
			if err != nil {
				fmt.Printf("Encode of write reply error: %s\n", err)
				return
			}
		case ExitOp:
			enc.Encode(reply{ExitReply, op.Size})
			return
		}
	}
}
