//go:build linux
// +build linux

package executor_test

import (
	"nightcord-server/internal/conf"
	"nightcord-server/internal/model"
	"nightcord-server/internal/service/executor"
	"os"
	"sync"
	"testing"
)

func TestProcess(t *testing.T) {
	conf.InitConfig()
	req := model.SubmitRequest{
		SourceCode:     "#include <stdio.h>\n\nint main(void) {\n  char name[10];\n  scanf(\"%s\", name);\n  printf(\"hello, %s\\n\", name);\n  return 0;\n}",
		LanguageID:     1,
		Stdin:          "world\n",
		ExpectedOutput: "hello, world\n",
	}
	res := executor.GetJobManagerInstance().SubmitJob(req)

	t.Logf("%+v", res)
}

func TestExecutor(t *testing.T) {
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inW.Close()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outR.Close()
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer errR.Close()
	e := model.Executor{
		Command: "./test_a+b",
		Dir:     ".",
		Limiter: model.Limiter{
			CpuTime: 1,
			Memory:  102400,
		},
		Stdin:   inR,
		Stdout:  outW,
		Stderr:  errW,
		RunFlag: true,
	}
	inW.Write([]byte("5 2\n"))
	var wg sync.WaitGroup
	wg.Add(2)

	// 异步读取标准输出
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		n, _ := outR.Read(buf)
		t.Logf("stdout: %s", string(buf[:n]))
	}()

	// 异步读取标准错误
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		n, _ := errR.Read(buf)
		t.Logf("stderr: %s", string(buf[:n]))
	}()

	// 执行命令
	res, err := executor.ProcessExecutor(e)
	if err != nil {
		t.Fatal(err)
	}

	// 显式关闭写入端
	inR.Close()
	outW.Close()
	errW.Close()

	wg.Wait()
	t.Logf("%+v", res)

}
