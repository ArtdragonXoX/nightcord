#ifndef EXECUTOR_H
#define EXECUTOR_H
#include <stdlib.h>
#include <unistd.h>
#include <sys/resource.h>
#include <sys/wait.h>
#include <signal.h>
#include <seccomp.h>
#include <sys/syscall.h>
#include <sys/prctl.h>

// Result 表示执行结果
typedef struct
{
    float Time;
    int Memory;
    int Signal;
    int ExitCode;
} Result;

// Limiter 表示限制条件
typedef struct
{
    float CpuTime_cur; // s
    float CpuTime_max;
    int Memory_cur; // kb
    int Memory_max;
} Limiter;

// Executor 表示运行器
typedef struct
{
    char *Command;
    char *Dir;
    Limiter Limit;
    Result Result;
    int StdinFd;
    int StdoutFd;
    int StderrFd;
    int RunFlag;
} Executor;

// Execute 执行运行器
int Execute(Executor *executor);

// InitFilter 初始化过滤器
void InitFilter();

#endif