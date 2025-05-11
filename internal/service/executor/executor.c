#include "executor.h"

// 运行子进程过滤器
scmp_filter_ctx runSeccompFilter;
// 编译子进程过滤器
scmp_filter_ctx compileSeccompFilter;

// 设置seccomp过滤器
void setupSeccomp(scmp_filter_ctx ctx)
{
    // 加载seccomp过滤器
    if (seccomp_load(ctx) != 0)
    {
        perror("seccomp_load failed");
        exit(2);
    }
}

/**
 * 设置进程的资源限制。
 *
 * @param limiter 指向Limiter结构的指针，包含CPU时间和内存限制的配置值。
 * @return 无返回值。若设置失败则终止程序。
 */
void setLimits(Limiter *limiter)
{
    struct rlimit cpu_limit, mem_limit;

    /* 设置CPU时间限制（RLIMIT_CPU） */
    cpu_limit.rlim_cur = limiter->CpuTime_max;
    cpu_limit.rlim_max = limiter->CpuTime_max;
    if (setrlimit(RLIMIT_CPU, &cpu_limit) == -1)
    {
        perror("setrlimit(RLIMIT_CPU)");
        exit(2);
    }

    /* 设置虚拟内存使用限制（RLIMIT_AS），单位转换为字节 */
    mem_limit.rlim_cur = limiter->Memory_cur * 1024;
    mem_limit.rlim_max = limiter->Memory_max * 1024;
    if (setrlimit(RLIMIT_AS, &mem_limit) == -1)
    {
        perror("setrlimit(RLIMIT_AS)");
        exit(2);
    }

    /* 设置最大文件描述符数限制（RLIMIT_NOFILE） */
    struct rlimit nofile_limit = {1024, 1024}; // 最大文件描述符数
    if (setrlimit(RLIMIT_NOFILE, &nofile_limit) == -1)
    {
        perror("setrlimit(RLIMIT_NOFILE)");
        exit(2);
    }

    /* 禁用核心转储（RLIMIT_CORE） */
    struct rlimit core_limit = {0, 0}; // 禁用核心转储
    if (setrlimit(RLIMIT_CORE, &core_limit) == -1)
    {
        perror("setrlimit(RLIMIT_CORE)");
        exit(2);
    }
}

/*
 * 函数名：childProcess
 * 参数：Executor *executor - 指向执行器结构体的指针，包含执行命令所需的各种配置（如文件描述符、目录、资源限制等）
 * 返回值：int - 退出状态码（实际通过_exit()退出，返回值由_exit参数决定）
 * 功能描述：在子进程中执行必要的初始化操作并启动指定的命令执行。主要步骤包括重定向标准输入输出、切换工作目录、设置资源限制和安全限制，最后通过shell执行命令。
 */
int childProcess(Executor *executor)
{
    // 重定向标准错误输出到executor指定的文件描述符，并关闭原始描述符
    if (dup2(executor->StderrFd, STDERR_FILENO) == -1)
    {
        perror("dup2(STDERR_FILENO)");
        _exit(3);
    }
    close(executor->StderrFd);

    // 如果指定工作目录，切换到该目录
    if (executor->Dir != NULL)
    {
        if (chdir(executor->Dir) == -1)
        {
            perror("chdir");
            _exit(2);
        }
    }

    // 重定向标准输入到executor指定的文件描述符，并关闭原始描述符
    if (dup2(executor->StdinFd, STDIN_FILENO) == -1)
    {
        perror("dup2(STDIN_FILENO)");
        _exit(2);
    }
    close(executor->StdinFd);

    // 重定向标准输出到executor指定的文件描述符，并关闭原始描述符
    if (dup2(executor->StdoutFd, STDOUT_FILENO) == -1)
    {
        perror("dup2(STDOUT_FILENO)");
        _exit(2);
    }
    close(executor->StdoutFd);

    // 应用资源限制配置
    setLimits(&executor->Limit);

    // 根据执行模式设置seccomp安全过滤器（运行模式或编译模式）
    setupSeccomp(executor->RunFlag ? runSeccompFilter : compileSeccompFilter);

    // 禁止进程后续获得新权限
    if (prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0) == -1)
    {
        perror("prctl(PR_SET_NO_NEW_PRIVS)");
        _exit(2);
    }

    // 执行指定的命令字符串，通过shell解释执行
    execl("/bin/sh", "sh", "-c", executor->Command, (char *)NULL);
    perror("execl fail");
    _exit(2);
}

/**
 * 执行命令并管理子进程。
 *
 * @param executor 指向执行器的指针，包含执行所需参数及结果存储结构。
 * @return
 *   - EXIT_SUCCESS 执行成功
 *   - EXIT_FAILURE 子进程执行失败或wait4调用失败
 *   - 1 fork()系统调用失败
 */
int Execute(Executor *executor)
{
    pid_t pid = fork();

    /* 子进程逻辑：执行指定命令 */
    if (pid == 0)
    {
        childProcess(executor);
    }

    /* 父进程逻辑：等待子进程结束并收集结果 */
    else if (pid > 0)
    {
        int status;
        struct rusage usage;
        int ret = wait4(pid, &status, 0, &usage); // 保存返回值

        /* wait4调用失败，返回执行失败状态 */
        if (ret == -1)
        {
            return EXIT_FAILURE;
        }

        /* 解析子进程退出状态并保存到执行器结果 */
        executor->Result.ExitCode = WEXITSTATUS(status);
        executor->Result.Signal = WTERMSIG(status);
        executor->Result.Time = usage.ru_utime.tv_sec + usage.ru_utime.tv_usec / 1000000.0;
        executor->Result.Memory = usage.ru_maxrss;
    }

    /* fork()调用失败，返回错误码1 */
    else
    {
        return 1;
    }

    return EXIT_SUCCESS;
}

// 获取运行子进程过滤器
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

// 获取编译子进程过滤器
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

// 初始化全局变量的过滤器
void InitFilter()
{
    runSeccompFilter = getRunSeccompFilter();
    compileSeccompFilter = getCompileSeccompFilter();
    return;
}