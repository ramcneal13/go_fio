[global]
version=1
verbose
;linear=1us, 20us, 1us
job-order= read1, barrier, read1, read2

[job "read1"]
runtime=30s
size=5g
iodepth=8
access-pattern=100:rw:4k
;verbose
name=$filename

[job "read2"]
runtime=30s
size=5g
iodepth=8
access-pattern=100:rw:4k
;verbose
name=$filename1
