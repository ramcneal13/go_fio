package main

import (
	"flag"
	"os"
	"rmcneal.com/support"
	"runtime"
)

var inputFile string

func init() {
	const (
		defaultFile = "fio.j"
		usage       = "File containing job instructions"
	)
	flag.StringVar(&inputFile, "jobs_file", defaultFile, usage)
	flag.StringVar(&inputFile, "j", defaultFile, usage+" (shorthand)")
}

func main() {
	var cfg *support.Configs
	var err error
	var stats *support.StatsState = nil

	jobs := map[string]*support.Job{}

	// All early returns are error conditions so the default will
	// be an non-zero exit code. Only at the end after all tests
	// have been completed successfully will the exitCode be set
	// to 0.
	exitCode := 1

	flag.Parse()
	printer := support.PrintInit()

	defer func() {
		if stats != nil {
			stats.Send(support.StatsRecord{OpType: support.StatStop})
			stats.Flush()
		}
		printer.Exit()
		os.Exit(exitCode)
	}()

	if cfg, err = support.ReadConfig(inputFile); err != nil {
		printer.Send("Config failure: %s\n", err)
		return
	}

	stats, err = support.StatsInit(&cfg.Global, printer)
	if err != nil {
		printer.Send("Failure to start stat engine: %s\n", err)
		return
	}

	if cfg.Global.Verbose {
		titleCol := 0
		for _, v := range []int{len("version"), len("intermediate-stats"), len("job-order"),
			len("GOMAXPROCS")} {
			if titleCol < v {
				titleCol = v
			}
		}
		printer.Send("%*s: %d\n%*s: %d\n%*s: %s\n%*s: %s\n",
			titleCol, "GOMAXPROCS", runtime.GOMAXPROCS(-1),
			titleCol, "version", cfg.Global.Version,
			titleCol, "intermediate-stats", cfg.Global.Intermediate_Stats,
			titleCol, "job-order", cfg.Global.Job_Order)
	}

	track := support.TrackingInit(printer)
	for _, perBarrier := range *cfg.GetBarrierOrder() {

		for _, name := range perBarrier {
			if jd, ok := cfg.Job[name]; !ok {
				printer.Send("\nBad name in job list -- '%s'\n", name)
			} else {
				if job, err := support.JobInit(name, jd, stats); err != nil {
					printer.Send("[%s] %s\n", name, err)
					return
				} else {
					jobs[name] = job
				}
				if cfg.Global.Verbose {
					printer.Send("---- [%s] ----\n", name)
					support.DisplayInterface(cfg.Job[name], printer)
				}
			}
		}

		track.SetTitle("Preparing")
		if len(perBarrier) < 10 {
			track.DisplayExtra()
		} else {
			track.DisplayCount()
		}

		for _, name := range perBarrier {
			job := jobs[name]
			track.RunFunc(name, func() bool {
				if err := job.FillAsNeeded(track); err == nil {
					return true
				} else {
					printer.Send("\nERROR: [%s] %s\n", job.GetName(), err)
					return false
				}
			}, func() { job.AbortPrep() })
		}
		if !track.WaitForThreads() {
			break
		}
		track.DisplayReset()

		// Clear out the stats just before starting the jobs. The timer is running
		// in the stats thread which means the time spent during the prepare phase
		// would be counted against the elapsed time for these threads if we don't
		// clear the stats now.
		stats.Send(support.StatsRecord{OpType: support.StatClear})

		track.SetTitle("Run")
		track.DisplaySet(func() { printer.Send(stats.String()) })

		for _, name := range perBarrier {
			job := jobs[name]
			track.RunFunc(name, func() bool {
				job.Start()
				return true
			}, func() { job.Stop() })
		}
		track.WaitForThreads()
		track.DisplayReset()
		stats.Send(support.StatsRecord{OpType: support.StatDisplay})
		stats.Flush()

		track.SetTitle("Clean up")
		track.DisplayCount()
		for _, name := range perBarrier {
			job := jobs[name]
			track.RunFunc(name, func() bool {
				job.Fini()
				return true
			}, nil)
		}
		track.WaitForThreads()
	}
	exitCode = 0
}
