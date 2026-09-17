//go:build !windows

package main

import "errors"

func isWindowsService() bool { return false }

func runAsService(dataDir, listen string) {}

func serviceCommand(args []string) error {
	return errors.New("the service commands are for Windows. On Linux use the systemd unit in deploy/linux (see docs/INSTALL-SERVER.md)")
}
