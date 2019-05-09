package main

import (
	"flag"
	"fmt"
	"gopkg.in/gcfg.v1"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

var inputFile string

func init() {
	const (
		defaultFile = "fiod_config"
		usage       = "Configuration file for automated fiod"
	)
	flag.StringVar(&inputFile, "config_file", defaultFile, usage)
	flag.StringVar(&inputFile, "c", defaultFile, usage+" (shorthand)")
}

//noinspection GoSnakeCaseUsage
type OptionsData struct {
	Access_Pattern string
	Directory      string
	Num_Files      string
	Runtime        string
	Verbose        bool
	Config_File    string
	Size           string
	Iodepth        string

	rn *rand.Rand
}

type Config struct {
	Config OptionsData
}

func main() {
	var cfg *Config
	var err error
	var of *os.File

	flag.Parse()

	if cfg, err = readConfig(inputFile); err != nil {
		fmt.Printf("Configuration file error: %s\n", err)
		os.Exit(1)
	}

	if of, err = os.OpenFile(cfg.Config.Config_File, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666); err != nil {
		fmt.Printf("Failed to open output file %s, err=%s", cfg.Config.Config_File, err)
		os.Exit(1)
	}

	iodepth := cfg.Config.selectIODepth()
	_, _ = fmt.Fprintf(of, "[global]\nversion=1\ndirectory=%s\naccess-pattern=%s\niodepth=%d\n",
		cfg.Config.Directory, cfg.Config.Access_Pattern, iodepth)
	if cfg.Config.Verbose {
		_, _ = fmt.Fprintf(of, "verbose\n")
	}

	max := cfg.Config.selectNumFiles()
	if max*iodepth >= 1000 {
		fmt.Printf("num-files * iodepth can't exceed 1,000. Thread limit")
		os.Exit(1)
	}

	for i := 0; i < max; i++ {
		_, _ = fmt.Fprintf(of, "\n[job \"j%d\"]\nname=file-%d\nsize=%s\nruntime=%s\n", i, i,
			cfg.Config.selectSize(), cfg.Config.selectRuntime())
	}

	_ = of.Close()
}

func readConfig(f string) (*Config, error) {
	cfg := &Config{}

	if err := gcfg.ReadFileInto(cfg, f); err != nil {
		return nil, err
	}

	if err := cfg.Config.validate(); err != nil {
		return nil, err
	}

	cfg.Config.rn = rand.New(rand.NewSource(time.Now().UnixNano()))
	return cfg, nil
}

//
// Case #1
// start-end:optional
// Randomly selects value between start and end, converts said value to a string, and adds
// "optional" character(s) to return string
//
// Case #2
// 1, 3, 5, 7, 11
// List of values or strings separated by commas. One of which is randomly selected.
//
// Case #3
// Fall through from the first and it's just a single item which is returned
//
func (o *OptionsData) selectFromRange(s string) string {
	r := strings.Split(s, "-")

	if len(r) != 1 {
		var opt string
		var end int
		var val int
		if len(r) != 2 {
			fmt.Printf("Invalid range of %s, should be start-end:optional\n", s)
			return ""
		}
		start, _ := strconv.Atoi(r[0])
		eStr := strings.Split(r[1], ":")
		end, _ = strconv.Atoi(eStr[0])
		if len(eStr) == 1 {
			opt = ""
		} else {
			opt = eStr[1]
		}
		val = int(o.rn.Int31n(int32(end)-int32(start))) + start
		return fmt.Sprintf("%d%s", val, opt)
	} else {
		r = strings.Split(s, ",")
		if len(r) == 1 {
			return s
		}
		idx := o.rn.Int31n(int32(len(r)))
		return r[idx]
	}
}

func (o *OptionsData) selectNumFiles() int {
	v, _ := strconv.Atoi(o.selectFromRange(o.Num_Files))
	return v
}

func (o *OptionsData) selectSize() string {
	return o.selectFromRange(o.Size)
}

func (o *OptionsData) selectRuntime() string {
	return o.selectFromRange(o.Runtime)
}

func (o *OptionsData) selectIODepth() int {
	v, _ := strconv.Atoi(o.selectFromRange(o.Iodepth))
	return v
}

func (o *OptionsData) validate() error {
	if o.Config_File == "" {
		return fmt.Errorf("missing config-file setting")
	}

	if o.Access_Pattern == "" {
		o.Access_Pattern = "100:rw:8k"
	}
	if o.Num_Files == "" {
		o.Num_Files = "10"
	}
	if o.Runtime == "" {
		o.Runtime = "10m"
	}
	if o.Directory == "" {
		o.Directory = "./"
	}
	if o.Size == "" {
		o.Size = "100m"
	}
	if o.Iodepth == "" {
		o.Iodepth = "1"
	}

	return nil
}
