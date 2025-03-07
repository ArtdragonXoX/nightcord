#include "executor.h"

void setupSeccomp()
{
    scmp_filter_ctx ctx = seccomp_init(SCMP_ACT_KILL);
    if (!ctx)
    {
        perror("seccomp_init failed");
        exit(2);
    }

    // 定义允许的系统调用
    int allowCalls[] = {
        SCMP_SYS(read),  // 标准输入
        SCMP_SYS(write), // 标准输出/错误
        SCMP_SYS(exit),  //
        SCMP_SYS(exit_group),
        SCMP_SYS(brk), // 内存管理
        SCMP_SYS(mmap),
        SCMP_SYS(munmap),
        SCMP_SYS(fstat),
        SCMP_SYS(arch_prctl), // 部分语言需要（如Rust）
        SCMP_SYS(clock_gettime),
        SCMP_SYS(rt_sigreturn),
    };

    for (int i = 0; i < sizeof(allowCalls) / sizeof(allowCalls[0]); i++)
    {
        if (seccomp_rule_add(ctx, SCMP_ACT_ALLOW, allowCalls[i], 0) != 0)
        {
            perror("seccomp_rule_add failed");
            exit(2);
        }
    }

    // 加载seccomp过滤器
    if (seccomp_load(ctx) != 0)
    {
        perror("seccomp_load failed");
        exit(2);
    }

    seccomp_release(ctx);
}

void setLimits(Limiter *limiter)
{
    struct rlimit cpu_limit, mem_limit;
    cpu_limit.rlim_cur = limiter->CpuTime_cur;
    cpu_limit.rlim_max = limiter->CpuTime_max;
    if (setrlimit(RLIMIT_CPU, &cpu_limit) == -1)
    {
        perror("setrlimit(RLIMIT_CPU)");
        exit(2);
    }
    mem_limit.rlim_cur = limiter->Memory_cur * 1024;
    mem_limit.rlim_max = limiter->Memory_max * 1024;
    if (setrlimit(RLIMIT_AS, &mem_limit) == -1)
    {
        perror("setrlimit(RLIMIT_AS)");
        exit(2);
    }
}

int childProcess(Executor *executor)
{

    if (dup2(executor->StderrFd, STDERR_FILENO) == -1)
    {
        perror("dup2(STDERR_FILENO)");
        _exit(3);
    }
    close(executor->StderrFd);

    if (executor->Dir != NULL)
    {
        if (chdir(executor->Dir) == -1)
        {
            perror("chdir");
            _exit(2);
        }
    }

    if (dup2(executor->StdinFd, STDIN_FILENO) == -1)
    {
        perror("dup2(STDIN_FILENO)");
        _exit(2);
    }
    close(executor->StdinFd);

    if (dup2(executor->StdoutFd, STDOUT_FILENO) == -1)
    {
        perror("dup2(STDOUT_FILENO)");
        _exit(2);
    }
    close(executor->StdoutFd);

    setupSeccomp();
    setLimits(&executor->Limit);
    execl("/bin/sh", "sh", "-c", executor->Command, NULL);
    perror("execl fail");
    _exit(2);
}

int Execute(Executor *executor)
{
    pid_t pid = fork();
    if (pid == 0)
    {
        // 子进程
        childProcess(executor);
    }
    else if (pid > 0)
    {
        // 父进程
        int status;
        struct rusage usage;
        int ret = wait4(pid, &status, 0, &usage); // 保存返回值
        if (ret == -1)
        {
            return EXIT_FAILURE;
        }
        executor->Result.ExitCode = WEXITSTATUS(status);
        executor->Result.Signal = WTERMSIG(status);
        executor->Result.Time = usage.ru_utime.tv_sec + usage.ru_utime.tv_usec / 1000000.0;
        executor->Result.Memory = usage.ru_maxrss;
    }
    else
    {
        return 1;
    }
    return EXIT_SUCCESS;
}