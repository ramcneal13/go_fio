[global]
version=1
directory=.
;record-file=/Users/rmcneal/tmp/fiod/bw_record.csv
record-time=1s

; When outputing stats give the raw data as well as the human readable
; format. "verbose" can also be used at the per job level to see each I/O
; block, worker id, and read/write data. Used for code debug.
; verbose

; Due to limitation in INI processing the job order can't be determined
; by its position in this file. So, use job-order to specify the order
; in which jobs should be run.
job-order=read, randread, write, randwrite, rw

[job "rw"]
barrier
name=fubar
size=4g
block-size=8k
rw=rw
read-percent=40

[job "read"]
; runtime=1m
; Specify the file name. If not set the job name will be used instead.
name=fubar
; verbose

size=2g
block-size=8k

; fsync is the number of I/O's sent before calling sync. Default value
; is zero which means the system will used buffered I/O through.
fsync=64

; Number of outstanding I/O's for a given job.
iodepth=16

; Types of operations:
;   rw -- read/write i/o will be used in random order.
;   rwseq -- read/write i/o will be done sequential through the file
;   read -- read sequentially (default)
;   write -- write sequentially
;   randread -- reads randomly
;   randwrite -- writes randomly
rw=read

; Access pattern are sets of tuples. Each tuple describes the percentage
; of the disk to access, operation type such as sequential reads, and block
; size.
; Multiple patterns will be randomly selected based on their percentage.
; If the pattern or patterns don't equal 100% the remainding portition of
; the device/file will not be accessed.
; Read/write patterns such as rw and rwseq by default are 50/50. However if
; a '|' character follows the operation name the next number will be the 
; percentage of read operations.
;
; Possibilities like the following are possible.
; access-pattern=100:rw:8k
; access-pattern=10:read:4k,40:rw:8k,50:randwrite:16k
access-pattern=20:rw|60:4k,40:randread:8k

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

[job "randread"]
;barrier
name=fubar1
rw=randread
iodepth=16
;read-percent=40
block-pattern=lcg
size=2g
fsync=64
; runtime=2m
; delay-start=10s
; verbose

[job "write"]
barrier
name=fubar
rw=write
iodepth=16
;read-percent=40
block-pattern=lcg
size=2g
fsync=64
; runtime=2m
; rate=512
; verbose

[job "randwrite"]
barrier
name=fubar
rw=randwrite
iodepth=16
block-pattern=lcg
size=2g
fsync=64
