package main

import (
	"flag"
	"fmt"
	"gopkg.in/gcfg.v1"
	"os"
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
	Num_Files      int
	Runtime        string
	Verbose        bool
	Config_File    string
	Size           string
	Iodepth        int
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

	_, _ = fmt.Fprintf(of, "[global]\nversion=1\nruntime=%s\ndirectory=%s\naccess-pattern=%s\niodepth=%d\n",
		cfg.Config.Runtime, cfg.Config.Directory, cfg.Config.Access_Pattern, cfg.Config.Iodepth)
	if cfg.Config.Verbose {
		_, _ = fmt.Fprintf(of, "verbose\n")
	}
	for i := 0; i < cfg.Config.Num_Files; i++ {
		_, _ = fmt.Fprintf(of, "\n[job \"j%d\"]\nname=file-%d\nsize=%s\n", i, i, cfg.Config.Size)
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

	return cfg, nil
}

func (o *OptionsData) validate() error {
	if o.Config_File == "" {
		return fmt.Errorf("missing config-file setting")
	}

	if o.Access_Pattern == "" {
		o.Access_Pattern = "100:rw:8k"
	}
	if o.Num_Files == 0 {
		o.Num_Files = 10
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
	if o.Iodepth == 0 {
		o.Iodepth = 1
	}

	return nil
}
