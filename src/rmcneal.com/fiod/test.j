[global]
version=1
verbose

[job "rw"]
runtime=30s
access-pattern=100:rw:8k
size=1g
; verbose
name=/workspaces/fiod
