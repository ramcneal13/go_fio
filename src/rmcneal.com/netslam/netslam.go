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
var requestSizeStr string
var runTimeStr string
var readPercent int
var iodepthMax int

func init() {
	flag.BoolVar(&asDeamon, "d", false, "start as daemon")
	flag.StringVar(&clientHost, "h", "", "client host to contact")
	flag.StringVar(&requestSizeStr, "s", "1k", "Request size")
	flag.StringVar(&runTimeStr, "t", "1m", "Run time")
	flag.IntVar(&readPercent, "r", 50, "Read percentage")
	flag.IntVar(&iodepthMax, "D", 0, "number of outstanding requests")
}

var runTime time.Duration
var requestSize int64

func main() {
	var err error
	var ok bool
	flag.Parse()

	if runTime, err = time.ParseDuration(runTimeStr); err != nil {
		fmt.Printf("Invalid run time: %s, error is %s\n", runTimeStr, err)
		os.Exit(1)
	}
	if requestSize, ok = support.BlkStringToInt64(requestSizeStr); !ok {
		fmt.Printf("Failed to parse size request of %s\n", requestSizeStr)
		os.Exit(1)
	}

	if asDeamon {
		runDeamon()
	} else if clientHost == "" {
		fmt.Printf("Must have -h or -d minimum\n")
		flag.PrintDefaults()
		os.Exit(1)
	} else {
		fmt.Printf("Size: %s, Run Time: %s, iodepth: %d\n", support.Humanize(requestSize, 1),
			runTime, iodepthMax)
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
	Version    = 0x15
)

type request struct {
	Op   int
	Size int64
	Vers int
}
type reply struct {
	Op   int
	Size int64
}

func clientOp() {
	var wg sync.WaitGroup
	buf := make([]byte, requestSize)
	timeBomb := time.After(runTime)
	iodepth := make(chan int, 100)

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

	for i := 0; i < iodepthMax; i++ {
		iodepth <- 1
	}
	go clientReader(conn, requestSize, &wg, iodepth)
	if iodepthMax == 0 {
		for {
			select {
			case <-timeBomb:
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
	} else {
		for {
			select {
			case <-timeBomb:
				return
			case <-iodepth:
				if rand.Intn(100) > readPercent {
					enc.Encode(request{WriteOp, requestSize, Version})
					enc.Encode(&buf)
				} else {
					enc.Encode(request{ReadOp, requestSize, Version})
				}
			}
		}
	}
}

func clientReader(conn net.Conn, size int64, wg *sync.WaitGroup, iodepth chan int) {
	var lastVal, bw, lowBW, highBW int64 = 0, 0, 0, 0
	var reply reply
	buf := make([]byte, size)
	dec := gob.NewDecoder(conn)
	tick := time.Tick(time.Second)
	tickCount := 1
	var useNewLine bool

	defer func() {
		fmt.Printf("\nAverage: %s -- Range: %s to %s\n", support.Humanize(bw/int64(tickCount), 1),
			support.Humanize(lowBW, 1), support.Humanize(highBW, 1))
		wg.Done()
	}()

	stdinStat, _ := os.Stdout.Stat()
	if stdinStat.Mode().IsRegular() {
		useNewLine = true
	} else {
		useNewLine = false
	}

	for {
		select {
		case <-tick:
			currentVal := bw - lastVal
			lastVal = bw
			if currentVal > highBW {
				highBW = currentVal
			}
			if lowBW == 0 || currentVal < lowBW {
				lowBW = currentVal
			}
			tickCount += 1
			if useNewLine {
				fmt.Printf("%s: %s\n", support.SecsToHMSstr(tickCount), support.Humanize(currentVal, 1))
			} else {
				fmt.Printf("%s: %s\r", support.SecsToHMSstr(tickCount), support.Humanize(currentVal, 1))
			}

		default:
			err := dec.Decode(&reply)
			if err != nil {
				fmt.Printf("Reply reader header error: %s\n", err)
				return
			}
			switch reply.Op {
			case ReadReply:
				if iodepthMax != 0 {
					iodepth <- 1
				}
				err = dec.Decode(&buf)
				if err != nil {
					fmt.Printf("Reply error on read data: %s\n", err)
					return
				}
				bw += int64(reply.Size)
			case WriteReply:
				if iodepthMax != 0 {
					iodepth <- 1
				}
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
	var wg sync.WaitGroup

	fromClient := gob.NewDecoder(conn)
	errorEnc := gob.NewEncoder(conn)
	err := fromClient.Decode(&op)
	opType := make(chan int, 100)
	if err != nil {
		fmt.Printf("Decode error: %s\n", err)
		return
	}

	if op.Op != InitialOp {
		fmt.Print("App protocol error. First packet not an InitialOp\n")
		errorEnc.Encode(reply{ExitReply, 0})
		return
	} else if op.Vers != Version {
		fmt.Printf("App protocol error. Version mismatch. Got %d, Expected %d\n", op.Vers, Version)
		errorEnc.Encode(reply{ExitReply, 0})
		return
	}

	wg.Add(1)
	go serverSend(conn, opType, op.Size, &wg)

	defer wg.Wait()

	buf := make([]byte, op.Size)
	for {
		err := fromClient.Decode(&op)
		if err != nil {
			fmt.Printf("Decode op header error: %s\n", err)
			return
		}

		switch op.Op {
		case InitialOp:
			fmt.Printf("App protocol error in loop\n")
			return
		case ReadOp:
			opType <- op.Op
		case WriteOp:
			err = fromClient.Decode(&buf)
			if err != nil {
				fmt.Printf("Decode buf error: %s\n", err)
				return
			}
			opType <- op.Op
		case ExitOp:
			opType <- op.Op
			return
		}
	}
}

func serverSend(conn net.Conn, opType chan int, size int64, wg *sync.WaitGroup) {
	sending := gob.NewEncoder(conn)
	buf := make([]byte, size)

	defer func() {
		sending.Encode(reply{ExitReply, size})
		wg.Done()
	}()

	for {
		switch <-opType {
		case ReadOp:
			err := sending.Encode(reply{ReadReply, size})
			if err != nil {
				fmt.Printf("sending encode error of header: %s\n", err)
				return
			}
			err = sending.Encode(&buf)
			if err != nil {
				fmt.Printf("sending data error: %s\n", err)
				return
			}
		case WriteOp:
			err := sending.Encode(reply{WriteReply, size})
			if err != nil {
				fmt.Printf("sending encode write error: %s\n", err)
				return
			}
		case ExitOp:
			return
		}
	}
}
