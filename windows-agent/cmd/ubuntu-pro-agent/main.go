// Package main is the windows-agent entry point.
package main

import (
	"context"
	"os"
	"os/signal"
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

	if err := a.Run(); err != nil {
		log.Error(context.Background(), err)

		if a.UsageError() {
			return 2
		}
		return 1
	}

	return 0
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
