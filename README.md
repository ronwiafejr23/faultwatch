# faultwatch

A live page-fault monitor for Linux processes, written in Go.

It reads `/proc/<pid>/stat` and `/proc/<pid>/status` every second and prints:

- **MINFLT/s**: minor page faults. The page was not mapped yet, but no disk I/O was needed.
- **MAJFLT/s**: major page faults. The page had to be read from disk/SSD.
- **RSS**: pages actually in physical memory.
- **VSZ**: size of the whole virtual address space.

## Build and run

```bash
go build -o faultwatch .
gcc -O0 -o victim/victim victim/victim.c

./victim/victim 500        # terminal 1: prints its PID
./faultwatch <PID>         # terminal 2
```

## What you'll see

- VSZ is 500 MB right after malloc, but RSS stays near 0 (lazy allocation).
- Each first touch of a 4 KB page causes one minor fault: 500 MB / 4 KB = 128,000 faults.
- RSS climbs 50 MB per second as pages get mapped to frames.
