package executor_test

import (
	"nightcord-server/internal/service/executor"
	"testing"
)

func TestGetBPF(t *testing.T) {
	bpf, err := executor.GetBPFSockFprog()
	if err != nil {
		t.Errorf("GetBPFSockFprog() failed:%v", err)
	} else {
		t.Logf("GetBPFSockFprog() success:%v", bpf)
	}
}
