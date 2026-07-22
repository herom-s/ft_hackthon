#include <stdio.h>
#include <string.h>

extern void fizzbuzz(int n, char *out);

static int tests_run = 0;
static int tests_failed = 0;

#define TEST(name, expr) do { \
    tests_run++; \
    if (!(expr)) { \
        printf("  \u2717 %s\n", name); \
        tests_failed++; \
    } else { \
        printf("  \u2713 %s\n", name); \
    } \
} while(0)

int main(void) {
    char buf[4096];

    printf("[FizzBuzz]\n");
    memset(buf, 0, sizeof(buf));
    fizzbuzz(15, buf);
    TEST("fizzbuzz(15)", strcmp(buf, "FizzBuzz") == 0);
    memset(buf, 0, sizeof(buf));
    fizzbuzz(3, buf);
    TEST("fizzbuzz(3)", strcmp(buf, "Fizz") == 0);
    memset(buf, 0, sizeof(buf));
    fizzbuzz(5, buf);
    TEST("fizzbuzz(5)", strcmp(buf, "Buzz") == 0);
    memset(buf, 0, sizeof(buf));
    fizzbuzz(7, buf);
    TEST("fizzbuzz(7)", strcmp(buf, "7") == 0);
    memset(buf, 0, sizeof(buf));
    fizzbuzz(1, buf);
    TEST("fizzbuzz(1)", strcmp(buf, "1") == 0);

    printf("\n%d tests, %d passed, %d failed\n", tests_run, tests_run - tests_failed, tests_failed);
    return tests_failed > 0 ? 1 : 0;
}
