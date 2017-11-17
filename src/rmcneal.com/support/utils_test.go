package support

import (
	"fmt"
	"strings"
	"testing"
)

func TestDashLine(t *testing.T) {
	match := "+-----+-----+"
	str := DashLine(5, 5)
	if strings.Compare(str, match) != 0 {
		t.Errorf("DashLine returned '%s', expected %s", str, match)
	}
}

func TestEnvStr(t *testing.T) {
	s := "/dev/rdsk/c0t'$disk'd0p0"
	repl, _ := EnvStrReplace(s)
	fmt.Printf("Will test EnvStrReplace(%s)\n  %s\n", s, repl)

	s2, _ := EnvStrReplace(s)
	if strings.Compare(s, s2) == 0 {
		t.Errorf("No replacement took place on %s", s2)
	}
	s = "$disk"
	repl, _ = EnvStrReplace(s)
	fmt.Printf("Will test EnvStrReplace(%s)\n  %s\n", s, repl)
	s2, _ = EnvStrReplace(s)
	if strings.Compare(s, s2) == 0 {
		t.Errorf("No replacement took place on %s", s2)
	}
}

type testStruct struct {
	First  int
	Second string
	third  int
}

func TestClearStruct(t *testing.T) {
	s := testStruct{13, "Hello", 0xbadcafe}
	integer := 20
	DebugEnable()
	ClearStruct(&s)
	if s.First != 0 {
		t.Errorf("ClearStruct failed on struct\n")
	}
	DebugDisable()
	ClearStruct(&integer)
	if integer != 0 {
		t.Errorf("ClearStruct failed on int type\n")
	}
}
