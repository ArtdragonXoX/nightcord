//go:build linux
// +build linux

package main

import (
	"nightcord-server/internal/bootstrap"
)

func main() {
	bootstrap.Init()
}
