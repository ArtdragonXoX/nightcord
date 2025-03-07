//go:build linux
// +build linux

package executor_test

import (
	"nightcord-server/internal/model"
	"nightcord-server/internal/service/executor"
	"testing"
)

func TestExecutor(t *testing.T) {
	e := &model.Executor{}
	Bpf, err := executor.GetBPFSockFprog()
	if err != nil {
		t.Errorf("GetBPF error:%v", err)
	}
	e.Filter = Bpf
	e.Limiter.CpuTime = 1
	e.RunCmdStr = "echo hello"
	err = e.Start()
	if err != nil {
		t.Errorf("e start fail:%v", err)
		return
	} else {
		t.Log("e start")
	}
	err = e.Wait()
	if err != nil {
		t.Errorf("e wait fail:%v", err)
	} else {
		t.Logf("e wait %v", e.Result.Stdout)
	}
}
