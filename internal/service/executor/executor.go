//go:build linux
// +build linux

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
	"syscall"
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
		res.Status = model.StatusIE.GetStatus()
		msg := "language not found"
		res.Message = msg
		return res
	}

	// 生成随机文件夹（六位字母+数字）
	folderName := utils.RandomString(6)
	if err := os.Mkdir(folderName, 0755); err != nil {
		res.Status = model.StatusIE.GetStatus()
		msg := err.Error()
		res.Message = msg
		return res
	}
	// 评测结束后删除临时文件夹
	defer os.RemoveAll(folderName)

	// 将源代码写入文件
	sourceFilePath := filepath.Join(folderName, lang.SourceFile)
	if err := os.WriteFile(sourceFilePath, []byte(req.SourceCode), 0644); err != nil {
		res.Status = model.StatusIE.GetStatus()
		msg := err.Error()
		res.Message = msg
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
			res.CompileOutput = outputStr
			res.Status = model.StatusCE.GetStatus()
			msg := err.Error()
			res.Message = msg
			return res
		}
	}

	// 构建运行命令，设置内存限制（ulimit -v）和通过/usr/bin/time获取CPU时间及内存数据
	runCmdStr := fmt.Sprintf("ulimit -v %d; /usr/bin/time -f '__TIME__:%%S S,__MEM__:%%M KB' %s", req.MemoryLimit, lang.RunCmd)
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
		res.Status = model.StatusIE.GetStatus()
		msg := err.Error()
		res.Message = msg
		return res
	}
	stderrPipe, err := runCmd.StderrPipe()
	if err != nil {
		res.Status = model.StatusIE.GetStatus()
		msg := err.Error()
		res.Message = msg
		return res
	}

	if err := runCmd.Start(); err != nil {
		res.Status = model.StatusIE.GetStatus()
		msg := err.Error()
		res.Message = msg
		return res
	}

	stdoutBytes, _ := io.ReadAll(stdoutPipe)
	stderrBytes, _ := io.ReadAll(stderrPipe)

	err = runCmd.Wait()

	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				switch status.Signal() {
				case syscall.SIGSEGV:
					res.Status = model.StatusRESIGSEGV.GetStatus()
					msg := "内存段错误"
					res.Stderr = msg
					return res
				case syscall.SIGXFSZ:
					res.Status = model.StatusRESIGXFSZ.GetStatus()
					msg := "文件大小限制超出"
					res.Stderr = msg
					return res
				case syscall.SIGFPE:
					res.Status = model.StatusRESIGFPE.GetStatus()
					msg := "算术运算错误"
					res.Stderr = msg
					return res
				case syscall.SIGABRT:
					res.Status = model.StatusRESIGABRT.GetStatus()
					msg := "程序异常终止"
					res.Stderr = msg
					return res
				}
			}
		}
	}

	// 判断是否运行超时
	if ctx.Err() == context.DeadlineExceeded {
		res.Status = model.StatusTLE.GetStatus()
	} else if err != nil {
		res.Status = model.StatusRE.GetStatus()
		msg := err.Error()
		res.Stderr = msg
	} else {
		res.Status = model.StatusAC.GetStatus()
	}

	// 从stderr中提取CPU时间和内存信息（使用正则）
	stderrStr := string(stderrBytes)
	regex := regexp.MustCompile(`__TIME__:(?P<time>\d+\.\d{2}) S,__MEM__:(?P<memory>\d+) KB`)
	matches := regex.FindStringSubmatch(stderrStr)
	if len(matches) >= 3 {
		timeParts := strings.Split(matches[1], ":")
		if len(timeParts) == 2 {
			minutes, _ := strconv.Atoi(timeParts[0])
			seconds, _ := strconv.ParseFloat(timeParts[1], 64)
			res.Time = float64(minutes)*60 + seconds
		} else {
			res.Time = 0.0 // 格式错误处理
		}
		memInt, _ := strconv.Atoi(matches[2])
		res.Memory = memInt
		// 将提取的时间信息从stderr中移除
		stderrStr = regex.ReplaceAllString(stderrStr, "")
		stderrStr = strings.TrimSpace(stderrStr)
		if stderrStr == "" {
			res.Stderr = ""
		} else {
			res.Stderr = stderrStr
		}
	} else {
		// 如果未提取到信息，则用超时时间作为近似值
		res.Time = timeoutDuration.Seconds()
		res.Memory = 0
		if stderrStr == "" {
			res.Stderr = ""
		} else {
			res.Stderr = stderrStr
		}
	}

	res.Stdout = string(stdoutBytes)

	if res.Status.Id == model.StatusAC {
		if res.Time != 0.0 && res.Time > req.CpuTimeLimit {
			res.Status = model.StatusTLE.GetStatus()
		}
	}

	// 若预期输出不匹配且状态为AC，则返回 Wrong Answer
	if req.ExpectedOutput != "" && res.Stdout != req.ExpectedOutput && res.Status.Id == model.StatusAC {
		res.Status = model.StatusWA.GetStatus()
	}

	return res
}
