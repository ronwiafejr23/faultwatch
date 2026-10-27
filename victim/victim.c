// victim.c: a test program that causes page faults on purpose.
// Usage: ./victim [megabytes]   (default 500)
//
// 1. malloc reserves virtual memory but RAM is NOT used yet (lazy allocation).
// 2. Touching one byte per 4 KB page forces the OS to map a real frame
//    on first access -> one MINOR page fault per page.
// 3. It touches pages slowly so you can watch faultwatch in another terminal.
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

int main(int argc, char *argv[]) {
    size_t mb = (argc > 1) ? (size_t)atol(argv[1]) : 500;
    size_t bytes = mb * 1024 * 1024;
    long page = sysconf(_SC_PAGESIZE);
    size_t pages = bytes / page;

    printf("victim pid: %d\n", getpid());
    printf("allocating %zu MB = %zu pages of %ld bytes\n", mb, pages, page);

    char *buf = malloc(bytes);
    if (!buf) { perror("malloc"); return 1; }

    printf("malloc done. RSS has barely moved: no pages touched yet.\n");
    printf("start faultwatch now; touching begins in 5 seconds...\n");
    fflush(stdout);
    sleep(5);

    // Touch pages in 10 chunks, one chunk per second.
    size_t chunk = pages / 10;
    for (int c = 0; c < 10; c++) {
        for (size_t i = c * chunk; i < (c + 1) * chunk; i++)
            buf[i * page] = 1;          // first write to this page -> minor fault
        printf("touched %d%% of pages\n", (c + 1) * 10);
        fflush(stdout);
        sleep(1);
    }

    printf("all pages touched. holding memory 5 more seconds...\n");
    fflush(stdout);
    sleep(5);
    free(buf);
    return 0;
}
