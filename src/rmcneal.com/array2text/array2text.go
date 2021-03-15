package main

import (
	"flag"
	"os"
	"fmt"
	"bufio"
	"io"
	"strings"
	"strconv"
)

var inputFile string

func init() {
	flag.StringVar(&inputFile, "input", "", "input file to use")
	flag.StringVar(&inputFile, "i", "", "input file to use (shorthand)")
}

func main() {
	flag.Parse()

	if fp, err := os.Open(inputFile); err != nil {
		fmt.Printf("Failed to open %s, err=%s\n", inputFile, err)
		os.Exit(1)
	} else {
		if err := parseArray(fp); err != nil {
			fmt.Printf("%s\n", err)
		}
	}
}

func parseArray(fp *os.File) error {
	rb := bufio.NewReader(fp)
	lineNumber := 1
	for {
		inputLine, err := rb.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read, err=%s", err)
		}
		inputLine = strings.TrimSuffix(inputLine, "\n")
		ArrayOfStrings := strings.Split(inputLine, ",")

		if len(ArrayOfStrings) == 1 {
			fmt.Printf("%s\n", inputLine)
		} else {
			ArrayOfNumbers := make([]byte, 0, 32)

			for _, v := range ArrayOfStrings {
				v = strings.TrimSpace(v)
				val, err := strconv.ParseUint(v, 0, 8)
				if err != nil {
					fmt.Printf("Invalid number %s on line %d, err=%s\n", v, lineNumber, err)
				}
				ArrayOfNumbers = append(ArrayOfNumbers, byte(val))
			}
			dumpMemory(ArrayOfNumbers, len(ArrayOfNumbers), "  ")
		}
		lineNumber++
	}
	return nil
}

func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

//noinspection GoUnusedFunction
func max(a int, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func hexDump(buf []byte, n int, offset int64, offsetWidth int) {
	if offsetWidth != 0 {
		fmt.Printf("%0*x: ", offsetWidth, offset)
	}
	for byteIndex := 0; byteIndex < n; byteIndex += 1 {
		fmt.Printf("%02x", buf[byteIndex])
		if (byteIndex % 4) == 3 {
			fmt.Printf(" ")
		}
	}
	if n%4 != 0 {
		fmt.Printf(" ")
	}
}

func asciiDump(buf []byte, n int) {
	remainder := 16 - n
	fmt.Printf("%*s%*s    ", (remainder%4)*2, "", (remainder/4)*9, "")

	for byteIndex := 0; byteIndex < n; byteIndex += 1 {
		if buf[byteIndex] >= ' ' && buf[byteIndex] <= '~' {
			fmt.Printf("%c", buf[byteIndex])
		} else {
			fmt.Printf(".")
		}
	}
}

func isLineZeros(buf []byte, n int) bool {
	for byteIndex := 0; byteIndex < n; byteIndex += 1 {
		if buf[byteIndex] != 0 {
			return false
		}
	}
	return true
}

func dumpMemory(buf []byte, n int, prefix string) {
	ow := 8
	lastLineZero := false
	printContinue := true
	if n < 0x100 {
		ow = 2
	} else if n < 0x10000 {
		ow = 4
	}
	for offset := 0; offset < n; offset += 16 {
		if isLineZeros(buf[offset:], min(16, n-offset)) {
			// Even if the last couple of lines in the buffer are zero print out
			// the last line which shows the offset and contents .
			if offset+16 >= n {
				lastLineZero = false
			}

			if lastLineZero {
				if printContinue {
					fmt.Printf("%s        ....\n", prefix)
					printContinue = false
				}
				continue
			} else {
				lastLineZero = true
			}
		} else {
			lastLineZero = false
			printContinue = true
		}
		fmt.Printf("%s", prefix)
		dumpLine(buf[offset:], min(16, n-offset), int64(offset), ow)
	}
}

func dumpLine(buf []byte, n int, offset int64, offsetWidth int) {
	hexDump(buf, n, offset, offsetWidth)
	asciiDump(buf, n)
	fmt.Printf("\n")
}
