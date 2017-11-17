package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type Oper struct {
	Name	string
	Value	string
}

func main() {
	fmt.Printf("Hello\n")
	if conn, err := net.Dial("tcp", "127.0.0.1:4646"); err != nil {
		fmt.Printf("Failed to connect: err=%s\n", err)
		os.Exit(1)
	} else {
		op := Oper{Name: "start", Value: "job_name"}
		if b, err1 := json.Marshal(op); err1 != nil {
			fmt.Printf("Marshal failed: err1=%s\n", err1)
			os.Exit(1)
		} else {
			fmt.Printf("Marshalled data: %s\n", b)
			conn.Write(b)
		}
	}

}
