//go:build windows

package main

// service_windows.go - run as a proper Windows service, and install/remove it.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName    = "FoghornServer"
	serviceDisplay = "Foghorn Alert Server"
	serviceDesc    = "Sends pop-up alerts to desktop computers and hosts the Foghorn web console."
)

func isWindowsService() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

type foghornService struct {
	dataDir, listen string
}

func (s *foghornService) Execute(args []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runServer(ctx, s.dataDir, s.listen, false) }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case err := <-done:
			cancel()
			if err != nil {
				log.Printf("ERROR server stopped: %v", err)
				return false, 1
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
				}
				return false, 0
			}
		}
	}
}

func runAsService(dataDir, listen string) {
	if err := svc.Run(serviceName, &foghornService{dataDir: dataDir, listen: listen}); err != nil {
		log.Printf("ERROR service failed: %v", err)
		os.Exit(1)
	}
}

func serviceCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: foghorn-server service install|uninstall|start|stop|status")
	}
	switch args[0] {
	case "install":
		fl := flag.NewFlagSet("service install", flag.ExitOnError)
		dataDir := fl.String("data", defaultDataDir(), "data folder")
		fl.Parse(args[1:])
		return installService(*dataDir)
	case "uninstall":
		return uninstallService()
	case "start":
		return controlService(true)
	case "stop":
		return controlService(false)
	case "status":
		return printServiceStatus()
	}
	return fmt.Errorf("unknown service command %q", args[0])
}

func connectSCM() (*mgr.Mgr, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("cannot reach the Windows service manager - run this from an elevated (Administrator) prompt: %w", err)
	}
	return m, nil
}

func installService(dataDir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.Abs(exe); err != nil {
		return err
	}
	if dataDir, err = filepath.Abs(dataDir); err != nil {
		return err
	}
	m, err := connectSCM()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s, err := m.OpenService(serviceName); err == nil {
		s.Close()
		return fmt.Errorf("the %s service is already installed (use: foghorn-server service uninstall)", serviceName)
	}
	s, err := m.CreateService(serviceName, exe, mgr.Config{
		DisplayName:      serviceDisplay,
		Description:      serviceDesc,
		StartType:        mgr.StartAutomatic,
		DelayedAutoStart: false,
	}, "run", "-data", dataDir)
	if err != nil {
		return fmt.Errorf("cannot create the service: %w", err)
	}
	defer s.Close()
	// If it ever crashes, Windows brings it back after 10 seconds.
	_ = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}, 86400)
	if err := s.Start(); err != nil {
		return fmt.Errorf("service installed but it did not start - look in %s\\foghorn.log: %w", dataDir, err)
	}
	fmt.Printf("Installed and started the \"%s\" service.\nData folder: %s\n", serviceDisplay, dataDir)
	return nil
}

func uninstallService() error {
	m, err := connectSCM()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("the %s service is not installed", serviceName)
	}
	defer s.Close()
	stopAndWait(s)
	if err := s.Delete(); err != nil {
		return err
	}
	fmt.Println("Service removed. Your data folder has been left in place.")
	return nil
}

func stopAndWait(s *mgr.Service) {
	st, err := s.Control(svc.Stop)
	if err != nil {
		return
	}
	for i := 0; i < 40 && st.State != svc.Stopped; i++ {
		time.Sleep(250 * time.Millisecond)
		if st, err = s.Query(); err != nil {
			return
		}
	}
}

func controlService(start bool) error {
	m, err := connectSCM()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("the %s service is not installed", serviceName)
	}
	defer s.Close()
	if start {
		if err := s.Start(); err != nil {
			return err
		}
		fmt.Println("Service started.")
		return nil
	}
	stopAndWait(s)
	fmt.Println("Service stopped.")
	return nil
}

func printServiceStatus() error {
	m, err := connectSCM()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		fmt.Println("not installed")
		return nil
	}
	defer s.Close()
	st, err := s.Query()
	if err != nil {
		return err
	}
	names := map[svc.State]string{svc.Stopped: "stopped", svc.StartPending: "starting", svc.StopPending: "stopping", svc.Running: "running"}
	fmt.Println(names[st.State])
	return nil
}
