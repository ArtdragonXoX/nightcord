package executor

import (
	"context"
	"fmt"
	"io"
	"nightcord-server/internal/conf"
	"nightcord-server/internal/model"
	"nightcord-server/internal/service/language"
	"nightcord-server/utils"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func Init() {
	// 初始化任务队列
	jobQueue = make(chan *model.Job, 100)

	// 启动协程池，数量由配置决定
	for i := 0; i < conf.Conf.Executor.Pool; i++ {
		go worker(i, jobQueue)
	}
}

// worker 不断从jobQueue中取任务执行
func worker(id int, jobs <-chan *model.Job) {
	for job := range jobs {
		result := processJob(job.Request)
		job.RespChan <- result
	}
}

// processJob 执行一次代码评测
func processJob(req model.SubmitRequest) model.Result {
	var res model.Result
	// 查找对应的语言配置
	var lang model.Language
	found := false
	for _, l := range language.GetLanguages() {
		if l.ID == req.LanguageID {
			lang = l
			found = true
			break
		}
	}
	if !found {
		res.Status.Id = -1
		res.Status.Description = "语言不存在"
		msg := "语言不存在"
		res.Message = &msg
		return res
	}

	// 生成随机文件夹（六位字母+数字）
	folderName := utils.RandomString(6)
	if err := os.Mkdir(folderName, 0755); err != nil {
		res.Status.Id = -1
		res.Status.Description = "创建临时文件夹失败"
		msg := err.Error()
		res.Message = &msg
		return res
	}
	// 评测结束后删除临时文件夹
	defer os.RemoveAll(folderName)

	// 将源代码写入文件
	sourceFilePath := filepath.Join(folderName, lang.SourceFile)
	if err := os.WriteFile(sourceFilePath, []byte(req.SourceCode), 0644); err != nil {
		res.Status.Id = -1
		res.Status.Description = "写入源代码失败"
		msg := err.Error()
		res.Message = &msg
		return res
	}

	// 如语言配置中有编译命令，则先进行编译
	if strings.TrimSpace(lang.CompileCmd) != "" {
		// 这里将编译命令中的 %s 替换为空字符串（可扩展为传递其它参数）
		compileCmdStr := fmt.Sprintf(lang.CompileCmd, "")
		compileCmd := exec.Command("bash", "-c", compileCmdStr)
		compileCmd.Dir = folderName
		compileOutput, err := compileCmd.CombinedOutput()
		if err != nil {
			outputStr := string(compileOutput)
			res.CompileOutput = &outputStr
			res.Status.Id = 1
			res.Status.Description = "Compilation Error"
			msg := err.Error()
			res.Message = &msg
			return res
		}
	}

	// 构建运行命令，设置内存限制（ulimit -v）和通过/usr/bin/time获取CPU时间及内存数据
	runCmdStr := fmt.Sprintf("ulimit -v %d; /usr/bin/time -f '__TIME__:%%S,__MEM__:%%M' %s", req.MemoryLimit, lang.RunCmd)
	timeoutDuration := time.Duration((req.CpuTimeLimit + conf.Conf.Executor.ExtraCPUTime) * float64(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	runCmd := exec.CommandContext(ctx, "bash", "-c", runCmdStr)
	runCmd.Dir = folderName
	if req.Stdin != "" {
		runCmd.Stdin = strings.NewReader(req.Stdin)
	}

	// 获取标准输出和标准错误
	stdoutPipe, err := runCmd.StdoutPipe()
	if err != nil {
		res.Status.Id = -1
		res.Status.Description = "获取stdout失败"
		msg := err.Error()
		res.Message = &msg
		return res
	}
	stderrPipe, err := runCmd.StderrPipe()
	if err != nil {
		res.Status.Id = -1
		res.Status.Description = "获取stderr失败"
		msg := err.Error()
		res.Message = &msg
		return res
	}

	if err := runCmd.Start(); err != nil {
		res.Status.Id = -1
		res.Status.Description = "启动运行命令失败"
		msg := err.Error()
		res.Message = &msg
		return res
	}

	stdoutBytes, _ := io.ReadAll(stdoutPipe)
	stderrBytes, _ := io.ReadAll(stderrPipe)

	err = runCmd.Wait()

	// 判断是否运行超时
	if ctx.Err() == context.DeadlineExceeded {
		res.Status.Id = 4
		res.Status.Description = "Time Limit Exceeded"
		msg := "执行超时"
		res.Message = &msg
	} else if err != nil {
		res.Status.Id = 2
		res.Status.Description = "Runtime Error"
		msg := err.Error()
		res.Message = &msg
	} else {
		res.Status.Id = 3
		res.Status.Description = "Accepted"
	}

	// 从stderr中提取CPU时间和内存信息（使用正则）
	stderrStr := string(stderrBytes)
	regex := regexp.MustCompile(`__TIME__:(?P<time>[0-9.]+),__MEM__:(?P<memory>[0-9]+)`)
	matches := regex.FindStringSubmatch(stderrStr)
	if len(matches) >= 3 {
		res.Time = matches[1]
		memInt, _ := strconv.Atoi(matches[2])
		res.Memory = memInt
		// 将提取的时间信息从stderr中移除
		stderrStr = regex.ReplaceAllString(stderrStr, "")
		stderrStr = strings.TrimSpace(stderrStr)
		if stderrStr == "" {
			res.Stderr = nil
		} else {
			res.Stderr = &stderrStr
		}
	} else {
		// 如果未提取到信息，则用超时时间作为近似值
		res.Time = fmt.Sprintf("%.3f", timeoutDuration.Seconds())
		res.Memory = 0
		if stderrStr == "" {
			res.Stderr = nil
		} else {
			res.Stderr = &stderrStr
		}
	}

	res.Stdout = string(stdoutBytes)

	// 若预期输出不为空但不匹配，则返回 Wrong Answer
	if req.ExpectedOutput != "" && res.Stdout != req.ExpectedOutput {
		res.Status.Id = 5
		res.Status.Description = "Wrong Answer"
	}

	return res
}
