//go:build linux
// +build linux

package bootstrap

import "nightcord-server/internal/service/executor"

func initExecutor() {
	executor.Init()
}
