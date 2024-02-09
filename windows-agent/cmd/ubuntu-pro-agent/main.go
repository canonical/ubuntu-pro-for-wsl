// Package main is the windows-agent entry point.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/cmd/ubuntu-pro-agent/agent"
	log "github.com/sirupsen/logrus"
)

//go:generate go generate ../../generate/...

func main() {
	i18n.InitI18nDomain(common.TEXTDOMAIN)
	a := agent.New()
	os.Exit(run(a))
}

type app interface {
	Run() error
	UsageError() bool
	Quit()
}

func run(a app) int {
	defer installSignalHandler(a)()

	log.SetFormatter(&log.TextFormatter{
		DisableLevelTruncation: true,
		DisableTimestamp:       true,

		// ForceColors is necessary on Windows, not only to have colors but to
		// prevent logrus from falling back to structured logs.
		ForceColors: true,
	})

	cleanup, err := setLoggerOutput()
	if err != nil {
		log.Warningf("could not set logger output: %v", err)
	} else {
		defer cleanup()
	}

	if err := a.Run(); err != nil {
		log.Error(context.Background(), err)

		if a.UsageError() {
			return 2
		}
		return 1
	}

	return 0
}

func setLoggerOutput() (func(), error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not find UserProfile: %v", err)
	} else if homeDir == "" {
		return nil, errors.New("could not find UserProfile: %USERPROFILE% is not set")
	}

	publicDir := filepath.Join(homeDir, common.UserProfileDir)
	if err := os.MkdirAll(publicDir, 0600); err != nil {
		return nil, errors.New("could not create logs dir")
	}

	logFile := filepath.Join(publicDir, "log")

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not open log file: %v", err)
	}

	fmt.Fprintf(f, "\n======== Startup %s ========\n", time.Now().Format(time.RFC3339))

	// Write both to file and to Stdout. The latter is useful for local development.
	w := io.MultiWriter(f, os.Stdout)
	log.SetOutput(w)

	return func() { _ = f.Close() }, nil
}

func installSignalHandler(a app) func() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			switch v, ok := <-c; v {
			case syscall.SIGINT, syscall.SIGTERM:
				a.Quit()
				return
			default:
				// channel was closed: we exited
				if !ok {
					return
				}
			}
		}
	}()

	return func() {
		signal.Stop(c)
		close(c)
		wg.Wait()
	}
}
