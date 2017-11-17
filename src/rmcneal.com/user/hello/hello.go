package main

import(
	"fmt"
	"runtime"
)

func main() {
	m := map[int]int{}
	m[0] = 1
	m[1] = 2
	fmt.Printf("Procs=%d\n", runtime.GOMAXPROCS(-1))
	fmt.Printf("NumCPU=%d\n", runtime.NumCPU())
	fmt.Printf("hello, world\n")	
	fmt.Printf("len(m) = %d\n", len(m))
	m[2] = 3
	fmt.Printf("len(m) = %d\n", len(m))
}
