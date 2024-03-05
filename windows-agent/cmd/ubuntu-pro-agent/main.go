// Package main is the windows-agent entry point.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

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
	PublicDir() (string, error)
}

func run(a app) int {
	defer installSignalHandler(a)()

	log.SetFormatter(&log.TextFormatter{
		DisableQuote: true,
	})

	cleanup, err := setLoggerOutput(a)
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

func setLoggerOutput(a app) (func(), error) {
	publicDir, err := a.PublicDir()
	if err != nil {
		return nil, err
	}

	logFile := filepath.Join(publicDir, "log")

	// Move old log file
	fileInfo, err := os.Stat(logFile)
	if err == nil && fileInfo.Size() > 0 {
		oldLogFile := filepath.Join(publicDir, "log.old")
		err = os.Rename(logFile, oldLogFile)
		if err != nil {
			log.Warnf("Could not archive previous log file: %v", err)
		}
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not open log file: %v", err)
	}

	// Write both to file and to Stdout. The latter is useful for local development.
	w := io.MultiWriter(f, os.Stdout)
	log.SetOutput(w)

	fmt.Fprintf(f, "\n======= STARTUP =======\n")
	log.Infof("Version: %s", common.Version)

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
