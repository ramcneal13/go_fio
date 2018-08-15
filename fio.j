[global]
version=1
directory=/Volumes/Workspaces/fio
;record-file=/Users/rmcneal/tmp/fiod/bw_record.csv
record-time=1s

; A histogram of the latency over the entire run is displayed at the
; end. By default the graph uses an exponential function to map the times.
; this way the user doesn't need to know anything about possible ranges.
; Optional linear can be used to define start, end, and interval values.
; The values have a time suffix like (ns, us, ms, s) so the following graph
; will at 4us and display up to 20us. Each bar will cover 1us in time.
; Values that fall outside of the range will be accumaled in the first
; or last buckets.
linear=4us, 20us, 1us

; When outputing stats give the raw data as well as the human readable
; format. "verbose" can also be used at the per job level to see each I/O
; block, worker id, and read/write data. Used for code debug.
verbose

; Due to limitation in INI processing the job order can't be determined
; by its position in this file. So, use job-order to specify the order
; in which jobs should be run.
; A special keyword 'barrier' can be used to run one or more jobs to completion
; before starting the next in line.
job-order=Reader, barrier, Bohica, Snafu

[job "Reader"]
runtime=1m
; Specify the file name. If not set the job name will be used instead.
name=fubar
; verbose

size=1g

; fsync is the number of I/O's sent before calling sync. Default value
; is zero which means the system will used buffered I/O through.
; fsync=64

; Number of outstanding I/O's for a given job.
iodepth=16

; Access Pattern
;   Made up of a tuple containing the percentage, operation, and block size
;   for the operation.
; Types of operations:
;   rw -- read/write i/o will be used in random order.
;   rwseq -- read/write i/o will be done sequential through the file
;   read -- read sequentially (default)
;   write -- write sequentially
;   randread -- reads randomly
;   randwrite -- writes randomly
;   none -- no i/o is done.
access-pattern=60:rw:8k,20:read:128k,20:rw|40:16k

; patterns available are:
;   zero -- fills the buffer with zeros,
;   rand -- uses Go's random number generator, expensive CPU
;   incr -- each byte is filled with its index value
;   lcg -- Linear Congruential Generator (default)
block-pattern=lcg

; Limit the job based on time instead of file size.
;runtime=2m

; Limit rate of I/O's issued during job. Doesn't work well due to limitation
; in Go's ability to sleep for subsecond periods.
; rate=512

[job "Bohica"]
name=bohica
access-pattern=100:rw|40:8k
iodepth=8
block-pattern=lcg
size=2g
fsync=64
; runtime=2m
; delay-start=10s
; verbose

[job "Snafu"]
name=snafu
access-pattern=100:rw:8k
iodepth=8
block-pattern=lcg
size=2g
fsync=64
; runtime=2m
; rate=512
; verbose
