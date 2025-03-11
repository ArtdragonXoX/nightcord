#include "executor.h"

scmp_filter_ctx runSeccompFilter;
scmp_filter_ctx compileSeccompFilter;

void setupSeccomp(scmp_filter_ctx ctx)
{
    // 加载seccomp过滤器
    if (seccomp_load(ctx) != 0)
    {
        perror("seccomp_load failed");
        exit(2);
    }
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

    struct rlimit nofile_limit = {1024, 1024}; // 最大文件描述符数
    if (setrlimit(RLIMIT_NOFILE, &nofile_limit) == -1)
    {
        perror("setrlimit(RLIMIT_NOFILE)");
        exit(2);
    }

    struct rlimit core_limit = {0, 0}; // 禁用核心转储
    if (setrlimit(RLIMIT_CORE, &core_limit) == -1)
    {
        perror("setrlimit(RLIMIT_CORE)");
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

    setLimits(&executor->Limit);
    setupSeccomp(executor->RunFlag ? runSeccompFilter : compileSeccompFilter);
    // 设置不允许获得新特权
    if (prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0) == -1)
    {
        perror("prctl(PR_SET_NO_NEW_PRIVS)");
        _exit(2);
    }
    execl("/bin/sh", "sh", "-c", executor->Command, (char *)NULL);
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

scmp_filter_ctx getRunSeccompFilter()
{
    scmp_filter_ctx ctx = seccomp_init(SCMP_ACT_ALLOW);
    if (!ctx)
    {
        perror("seccomp_init failed");
        exit(2);
    }
    int killCalls[] = {
        SCMP_SYS(kill),
        SCMP_SYS(tgkill),
        // SCMP_SYS(execve),
        SCMP_SYS(execveat),
        SCMP_SYS(clone),
        SCMP_SYS(fork),
        // SCMP_SYS(vfork),
        SCMP_SYS(open),
        // SCMP_SYS(openat),
        SCMP_SYS(openat2),
        SCMP_SYS(creat),
        SCMP_SYS(unlink),
        SCMP_SYS(unlinkat),
        SCMP_SYS(rename),
        SCMP_SYS(renameat),
        SCMP_SYS(mkdir),
        SCMP_SYS(rmdir),
        SCMP_SYS(chmod),
        SCMP_SYS(fchmod),
        SCMP_SYS(fchmodat),
        SCMP_SYS(chown),
        SCMP_SYS(fchown),
        SCMP_SYS(socket),
        SCMP_SYS(socketpair),
        SCMP_SYS(bind),
        SCMP_SYS(connect),
        SCMP_SYS(listen),
        SCMP_SYS(accept),
        SCMP_SYS(accept4),
        SCMP_SYS(getsockname),
        SCMP_SYS(getsockopt),
        SCMP_SYS(setsockopt),
        SCMP_SYS(sendto),
        SCMP_SYS(recvfrom),
        SCMP_SYS(sendmsg),
        SCMP_SYS(recvmsg),
        SCMP_SYS(ptrace),
        SCMP_SYS(mount),
        SCMP_SYS(umount),
        SCMP_SYS(umount2),
        SCMP_SYS(pivot_root),
        SCMP_SYS(chroot),
        SCMP_SYS(syslog),
        SCMP_SYS(kexec_load),
        SCMP_SYS(iopl),
        SCMP_SYS(ioperm),
        SCMP_SYS(shmget),
        SCMP_SYS(shmat),
        SCMP_SYS(shmdt),
        SCMP_SYS(msgget),
        SCMP_SYS(msgsnd),
        SCMP_SYS(msgrcv),
        SCMP_SYS(semget),
        SCMP_SYS(semop),
        SCMP_SYS(nanosleep),       // 禁止 nanosleep
        SCMP_SYS(clock_nanosleep), // 禁止 clock_nanosleep
    };
    for (int i = 0; i < sizeof(killCalls) / sizeof(killCalls[0]); i++)
    {
        if (seccomp_rule_add(ctx, SCMP_ACT_KILL, killCalls[i], 0) != 0)
        {
            perror("seccomp_rule_add failed");
            exit(2);
        }
    }
    return ctx;
}

scmp_filter_ctx getCompileSeccompFilter()
{
    scmp_filter_ctx ctx = seccomp_init(SCMP_ACT_ALLOW);
    if (!ctx)
    {
        perror("seccomp_init failed");
        exit(2);
    }
    int killCalls[] = {
        SCMP_SYS(kill),
        SCMP_SYS(tgkill),
        SCMP_SYS(socket),
        SCMP_SYS(socketpair),
        SCMP_SYS(bind),
        SCMP_SYS(connect),
        SCMP_SYS(listen),
    };
    for (int i = 0; i < sizeof(killCalls) / sizeof(killCalls[0]); i++)
    {
        if (seccomp_rule_add(ctx, SCMP_ACT_KILL, killCalls[i], 0) != 0)
        {
            perror("seccomp_rule_add failed");
            exit(2);
        }
    }
    return ctx;
}

void InitFilter()
{
    runSeccompFilter = getRunSeccompFilter();
    compileSeccompFilter = getCompileSeccompFilter();
    return;
}