package main

import (
	"flag"
	"os"
	"os/signal"
	"rmcneal.com/support"
	"runtime"
	"runtime/pprof"
	"time"
)

var inputFile string

func init() {
	const (
		defaultFile      = "fio.j"
		usage            = "File containing job instructions"
		defaultPprofFile = ""
		usagePprof       = "File with utilization info"
	)
	flag.StringVar(&inputFile, "jobs_file", defaultFile, usage)
	flag.StringVar(&inputFile, "j", defaultFile, usage+" (shorthand)")
}

func main() {
	var cfg *support.Configs
	var err error
	var pperfFp *os.File = nil
	var stats *support.StatsState = nil

	jobs := map[string]*support.Job{}
	jobExits := make(chan support.JobReport, 10)
	intrChans := make(chan os.Signal, 1)
	// All early returns are error conditions so the default will
	// be an non-zero exit code. Only at the end after all tests
	// have been completed successfully will the exitCode be set
	// to 0.
	exitCode := 1

	flag.Parse()
	printer := support.PrintInit()

	defer func() {
		if pperfFp != nil {
			pprof.StopCPUProfile()
		}
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
	signal.Notify(intrChans, os.Interrupt, os.Kill)
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
		printer.Send("%*s: %d\n", titleCol, "GOMAXPROCS", runtime.GOMAXPROCS(-1))
		printer.Send("%*s: %d\n", titleCol, "version", cfg.Global.Version)
		printer.Send("%*s: %s\n", titleCol, "intermediate-stats", cfg.Global.Intermediate_Stats)
		printer.Send("%*s: %s\n", titleCol, "job-order", cfg.Global.Job_Order)
	}

	track := support.TrackingInit(printer)
	for _, barrierGroups := range *cfg.GetBarrierOrder() {
		jobsStarted := 0
		track.SetTitle("Preparing")
		for _, name := range barrierGroups {
			if jd, ok := cfg.Job[name]; !ok {
				printer.Send("\nBad name in job list -- '%s'\n", name)
			} else {
				job := &support.Job{TargetName: name, JobParams: jd, Stats: stats}
				jobs[name] = job

				track.RunFunc(name, func() bool {
					if err := job.Init(track); err == nil {
						return true
					} else {
						printer.Send("[%s] %s\n", job.GetName(), err)
						return false
					}
				})
			}
		}
		track.WaitForThreads()
		if cfg.Global.Verbose {
			for _, name := range barrierGroups {
				support.DisplayInterface(cfg.Job[name], printer)
			}
		}

		printer.Send("Starting ... ")
		// Clear out the stats just before starting the jobs. The timer is running
		// in the stats thread which means the time spent during the prepare phase
		// would be counted against the elapsed time for these threads if we don't
		// clear the stats now.
		stats.Send(support.StatsRecord{OpType: support.StatClear})
		for _, name := range barrierGroups {
			if job, ok := jobs[name]; ok {
				printer.Send("[%s] ", name)
				go job.Start(jobExits)
				jobsStarted++
			} else {
				printer.Send("\nImpossible condition; job %s not initiazed\n", name)
				return
			}
		}
		printer.Send("\n")

		statMarker := time.Tick(cfg.GetIntermediateStats())
		// Now that the jobs have begun start displaying statistics on the output.
		stats.Send(support.StatsRecord{OpType: support.StatRelDisplay})
		runLoop := true
		for runLoop {
			select {
			case <-statMarker:
				stats.Send(support.StatsRecord{OpType: support.StatDisplay})

			case rpt := <-jobExits:
				if rpt.ReadErrors != 0 || rpt.WriteErrors != 0 {
					printer.Send("Job [%s]: Errors(read: %d, write: %d)\n", rpt.Name, rpt.ReadErrors, rpt.WriteErrors)
				}
				jobsStarted--
				if jobsStarted == 0 {
					runLoop = false
					break
				}

			case <-intrChans:
				printer.Send("Impatient\n")
				for _, name := range barrierGroups {
					if job, ok := jobs[name]; ok {
						job.Stop()
					}
				}
			}
		}
		printer.Send("\n")
		dumpHoldStats(stats)

		printer.Send("Clean up ... ")
		for _, name := range barrierGroups {
			if job, ok := jobs[name]; ok {
				printer.Send("[%s] ", job.GetName())
				job.Fini()
			}
		}
		printer.Send("\n")
	}
	exitCode = 0
}

func dumpHoldStats(stats *support.StatsState) {
	stats.Send(support.StatsRecord{OpType: support.StatHoldDisplay})
	stats.Send(support.StatsRecord{OpType: support.StatDisplay})
	stats.Send(support.StatsRecord{OpType: support.StatClear})
	stats.Flush()
}
