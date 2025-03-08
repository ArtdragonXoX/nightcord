#ifndef EXECUTOR_H
#define EXECUTOR_H
#include <stdlib.h>
#include <unistd.h>
#include <sys/resource.h>
#include <sys/wait.h>
#include <signal.h>
#include <seccomp.h>
#include <sys/syscall.h>

typedef struct
{
    float Time;
    int Memory;
    int Signal;
    int ExitCode;
} Result;

typedef struct
{
    float CpuTime_cur; // s
    float CpuTime_max;
    int Memory_cur; // kb
    int Memory_max;
} Limiter;

typedef struct
{
    char *Command;
    char *Dir;
    Limiter Limit;
    Result Result;
    int StdinFd;
    int StdoutFd;
    int StderrFd;
} Executor;

int Execute(Executor *executor);

void InitFilter();

#endif