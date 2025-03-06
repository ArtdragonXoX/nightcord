//go:build linux
// +build linux

package executor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"nightcord-server/utils"
	"os"
	"syscall"

	seccomp "github.com/seccomp/libseccomp-golang"
)

func GetBPFSockFprog() (BPF *syscall.SockFprog, err error) {
	filter, err := seccomp.NewFilter(seccomp.ActKillThread)
	if err != nil {
		return
	}
	defer filter.Release()

	// 允许必要的架构（必须）
	if err = filter.AddArch(seccomp.ArchAMD64); err != nil {
		return
	}

	// 允许OJ必须的系统调用白名单
	requiredCalls := []seccomp.ScmpSyscall{
		syscall.SYS_READ,  // 标准输入
		syscall.SYS_WRITE, // 标准输出/错误
		syscall.SYS_EXIT,
		syscall.SYS_EXIT_GROUP,
		syscall.SYS_BRK, // 内存管理
		syscall.SYS_MMAP,
		syscall.SYS_MUNMAP,
		syscall.SYS_FSTAT,
		syscall.SYS_ARCH_PRCTL, // 部分语言需要（如Rust）
		syscall.SYS_CLOCK_GETTIME,
		syscall.SYS_RT_SIGRETURN,
	}

	// 添加白名单规则
	for _, call := range requiredCalls {
		if err = filter.AddRule(call, seccomp.ActAllow); err != nil {
			return
		}
	}

	BpfFile, err := os.CreateTemp("", "bpf")

	if err != nil {
		return
	}

	defer func() {
		if err := BpfFile.Close(); err != nil {
			return
		}
	}()

	err = filter.ExportBPF(BpfFile)
	if err != nil {
		return
	}

	BpfData, err := io.ReadAll(BpfFile) // 使用os包函数读取
	if err != nil {
		return
	}
	BpfBytes := bytes.NewBuffer(BpfData)

	BpfFileStat, err := BpfFile.Stat()
	if err != nil {
		return
	}
	BpfFileSize := BpfFileStat.Size()
	if BpfFileSize%8 != 0 {
		err = errors.New("BpfSize is not divisible by 8")
		return
	}
	ret, err := BpfFile.Seek(0, io.SeekStart)
	if err != nil {
		return
	}
	if ret != 0 {
		err = errors.New("ret not at start")
		return
	}
	BpfFilter := make([]syscall.SockFilter, BpfFileSize/8)

	if utils.IsLittleEndian() {
		binary.Read(BpfBytes, binary.LittleEndian, &BpfFilter)
	} else {
		binary.Read(BpfBytes, binary.BigEndian, &BpfFilter)
	}
	BPF = &syscall.SockFprog{
		Len:    uint16(BpfFileSize / 8),
		Filter: &BpfFilter[0],
	}
	return
}
