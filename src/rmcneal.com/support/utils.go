// utils.go
package support

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
)

func Humanize(val int64, base int) string {
	idx := 0

	if base == 0 {
		base = 1
	}
	val *= int64(base)
	return humanizeImpl(float64(val), idx)
}

func humanizeImpl(val float64, idx int) string {
	suffix := " KMGTPE"

	switch {
	case val < 10.0:
		return fmt.Sprintf("%5.2f%c", val, suffix[idx])
	case val < 100.0:
		return fmt.Sprintf("%5.1f%c", val, suffix[idx])
	case val < 1024.0:
		return fmt.Sprintf("%5.0f%c", val, suffix[idx])
	default:
		return humanizeImpl(val/1024.0, idx+1)
	}
}

func SecsToHMSstr(seconds int) string {
	h := seconds / 3600
	m := seconds / 60 % 60
	s := seconds % 60
	return fmt.Sprintf("%02dh%02dm%02ds", h, m, s)
}

var debugOn = false
var debugIndex = 0

func DebugLog(format string, a ...interface{}) {
	if debugOn {
		fmt.Printf("%*s", debugIndex, "")
		fmt.Printf(format, a...)
	}
}
func DebugIncrease() {
	debugIndex += 4
}
func DebugDecrase() {
	debugIndex -= 4
}
func DebugEnable() {
	debugOn = true
}
func DebugDisable() {
	debugOn = false
}

func ClearStruct(v interface{}) {
	DebugLog("[]---- ClearStruct ----[]\n")
	DebugLog("Kind(%s), Type(%s)\n", reflect.ValueOf(v).Kind(), reflect.TypeOf(v))
	switch reflect.ValueOf(v).Kind() {
	case reflect.Ptr:
		s := reflect.ValueOf(v).Elem()
		if !s.CanSet() {
			DebugLog("CanSet is false for ptr\n")
			return
		}
		switch s.Kind() {
		case reflect.Struct:
			DebugIncrease()
			for i := 0; i < s.NumField(); i++ {
				f := s.Field(i)
				DebugLog("f.Name(%s) CanSet(%t), CanAddr(%t)\n", s.Type().Field(i).Name, f.CanSet(),
					f.CanAddr())
				if !f.CanSet() || !f.CanAddr() {
					continue
				}

				DebugIncrease()
				ClearStruct(s.Field(i).Addr().Interface())
				DebugDecrase()
			}
			DebugDecrase()
		case reflect.Int:
			s.SetInt(0)
		case reflect.Int64:
			s.SetInt(0)
		default:
			DebugLog("Unhandle Ptr(%s)\n", s.Kind())
		}
	case reflect.Struct:
		DebugLog("Struct: Name(%s), Type(%s), CanSet(%t)\n",
			reflect.TypeOf(v).Name(), reflect.ValueOf(v).Type(),
			reflect.ValueOf(v).CanSet())
	case reflect.Int:
		DebugLog("reflect.Int().CanSet(%t)\n", reflect.ValueOf(v).CanSet())
	case reflect.String:
		DebugLog("reflect.String().CanSet(%t)\n", reflect.ValueOf(v).CanSet())
	default:
		DebugLog("Kind(%s)=%d not handled\n",
			reflect.ValueOf(v).Kind(), reflect.ValueOf(v).Kind())
	}
}

//
// Need to change this to a state machine.
//
func EnvStrReplace(s string) (string, error) {
	var prevChar rune
	search := make([]byte, 0)
	quoteStart := false
	dollarStart := false

	prevChar = 0
	for _, char := range s {
		switch char {
		case '\'':
			if quoteStart {
				search = append(search, byte(char))
				r := os.Getenv(string(search[2 : len(search)-1]))
				s = strings.Replace(s, string(search), r, 1)
				quoteStart = false
			} else {
				quoteStart = true
				search = append(search, byte(char))
			}
		case '$':
			if quoteStart && (prevChar != '\'') {
				quoteStart = false
				search = nil
			}
			if quoteStart {
				search = append(search, byte(char))
			} else {
				search = append(search, byte(char))
				dollarStart = true
			}
		case ' ':
			if prevChar == '\'' {
				quoteStart = false
			}
			if quoteStart {
				search = append(search, byte(char))
			} else if dollarStart {
				// Don't include current space character in replacement
				// string like was done for " containment. For quotes
				// both delimiters should be replaced. Here it's just
				// the dollar sign and next word.
				r := os.Getenv(string(search[1:]))
				s = strings.Replace(s, string(search), r, -1)
				dollarStart = false
			}
		default:
			if prevChar == '\'' {
				quoteStart = false
			}
			if quoteStart || dollarStart {
				search = append(search, byte(char))
			}
		}
		prevChar = char
	}
	if dollarStart {
		r := os.Getenv(string(search[1:]))
		if r == "" {
			return "", fmt.Errorf("environment string \"%s\" missing", search[1:])
		}
		s = strings.Replace(s, string(search), r, -1)
	}
	return s, nil
}

func DisplayInterface(v interface{}, printer *Printer) {
	k := reflect.ValueOf(v).Kind()
	switch {
	case (k == reflect.Ptr) || (k == reflect.Interface):
		DisplayStruct(v, printer)
	default:
		printer.Send("v=%v\n", v)
	}
}

func DisplayStruct(v interface{}, printer *Printer) {
	maxName := 0
	maxValue := 0
	curValue := 0
	width := 0

	if ws, err1 := GetWinsize(os.Stdout.Fd()); err1 != nil {
		printer.Send("Failed to GetWinsize\n")
		width = 80
	} else {
		width = int(ws.Width)
	}

	s := reflect.ValueOf(v).Elem()
	typeOfT := s.Type()

	/*
	 * Find out the maximum width of the name/value pairs. Numbers take
	 * 8 columns on the screen whereas boolean takes up 5. For strings we
	 * simply find the length of the value string.
	 */
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if f.CanSet() {
			if len(typeOfT.Field(i).Name) > maxName {
				maxName = len(typeOfT.Field(i).Name)
			}
			switch f.Kind() {
			case reflect.Int:
				curValue = len(fmt.Sprintf("%d", f.Int()))
			case reflect.Bool:
				curValue = len(fmt.Sprintf("%t", f.Bool()))
			case reflect.String:
				curValue = len(f.String())
			}
			if curValue > maxValue {
				maxValue = curValue
			}
		}
	}

	columns := (width - 8) / (maxName + maxValue + 1)
	if columns == 0 {
		columns = 1
	}

	printer.Send("\t")
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if f.CanSet() {
			printer.Send("%*s:%*v", maxName, convertFieldName(typeOfT.Field(i).Name),
				maxValue, f.Interface())
			if ((i % columns) == (columns - 1)) && (i != (s.NumField() - 1)) {
				printer.Send("\n\t")
			}
		}
	}
	printer.Send("\n")
}

/*
 * Configuration names use values like "read-percentage". Those names can't be
 * used for structure members for two reasons. One is that the dash (-) is
 * not a valid character for a member name and the second reason is that
 * Go's introspection requires member names to be public so the names
 * read-percentage is Read_Percentage. To avoid confusion to someone running
 * the program besides me convert the names back to what they would expect
 * to see.
 */
func convertFieldName(s string) string {
	str := strings.ToLower(s)
	idx := strings.Index(str, "_")
	if idx != -1 {
		str = strings.Replace(str, "_", "-", -1)
	}
	return str
}

func DashLine(pos ...int) string {
	rval := "+"

	for i := range pos {
		for x := 0; x < pos[i]; x++ {
			rval += "-"
		}
		rval += "+"
	}
	return rval
}

func FindComma(r rune) bool {
	if r == rune(',') {
		return true
	} else {
		return false
	}
}

//
// Linear Congruential Generator
// https://en.wikipedia.org/wiki/Linear_congruential_generator
//
type RandLCG struct {
	lcgSeed int64
}

func (s *RandLCG) Init() {
	s.lcgSeed = time.Now().Unix()
}

func (s *RandLCG) RandSeed(seed int64) {
	s.lcgSeed = seed
}

func (s *RandLCG) Value(m int64) int64 {
	a := int64(1103515245)
	c := int64(12345)
	s.lcgSeed = (a*s.lcgSeed + c) % m
	return s.lcgSeed % m
}
