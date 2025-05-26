//go:build linux
// +build linux

package executor

import (
	"nightcord-server/internal/model"
	"syscall"
)

func SignalStatus(s syscall.Signal) model.StatusId {
	switch s {
	case syscall.SIGSEGV:
		return model.StatusRESIGSEGV
	case syscall.SIGXFSZ:
		return model.StatusRESIGXFSZ
	case syscall.SIGFPE:
		return model.StatusRESIGFPE
	case syscall.SIGABRT:
		return model.StatusRESIGABRT
	case syscall.SIGXCPU:
		return model.StatusTLE
	default:
		return model.StatusRE
	}
}

func SignalMessage(s syscall.Signal) string {
	switch s {
	case syscall.SIGSEGV:
		return "内存段错误"
	case syscall.SIGXFSZ:
		return "文件大小限制超出"
	case syscall.SIGFPE:
		return "算术运算错误"
	case syscall.SIGABRT:
		return "程序异常终止"
	case syscall.SIGSYS:
		return "系统调用错误"
	case syscall.SIGXCPU:
		return "时间限制超出"
	default:
		return "未知错误"
	}
}
