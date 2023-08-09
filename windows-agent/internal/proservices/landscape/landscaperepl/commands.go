package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape/landscapemockservice"
)

type command struct {
	callback func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool
	usage    string
	help     string
}

var commands map[string]command

func initCommands() {
	commands = map[string]command{
		"exit": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				return true
			},
			help: "exits the program",
		},
		"status": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) != 1 {
					fmt.Println("Wrong usage")
					printHelp("status")
					return false
				}

				hosts := s.Hosts()

				uid, err := uidRef(s, args[0])
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return false
				}

				host, ok := hosts[uid]
				if !ok {
					fmt.Println("HOST_UID not found")
					return false
				}

				fmt.Printf("uid:       %s\n", host.UID)
				fmt.Printf("hostname:  %s\n", host.Hostname)
				fmt.Printf("token:     %s\n", common.Obfuscate(host.Token))
				fmt.Printf("connected: %t\n", s.IsConnected(host.UID))

				fmt.Println("instances:")
				for _, d := range host.Instances {
					fmt.Printf(" - id:      %s\n", d.ID)
					fmt.Printf("   version: %s\n", d.VersionID)
					fmt.Printf("   state:   %s\n", d.InstanceState)
				}

				return false
			},
			usage: "status HOST_UID",
			help:  "Shows the status of the specified host",
		},
		"journal": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				for _, line := range s.MessageLog() {
					var instances []string
					for _, inst := range line.Instances {
						instances = append(instances, inst.ID)
					}

					fmt.Printf("UID: %s, Hostname: %q, Token: %q, Instances: %q\n", line.UID, line.Hostname, common.Obfuscate(line.Token), strings.Join(instances, ", "))
				}
				return false
			},
			help: "Prints the log",
		},
		"start": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) < 2 {
					fmt.Println("Wrong usage")
					printHelp("start")
					return false
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return false
				}

				for _, a := range args[1:] {
					err := s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_Start_{Start: &landscapeapi.Command_Start{Id: a}}})
					if err != nil {
						log.Printf("error: %v\n", err)
					}
				}
				return false
			},
			usage: "start HOST_UID INSTANCES...",
			help:  "Starts the specified instance(s) at the specified host",
		},
		"stop": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) < 2 {
					fmt.Println("Wrong usage")
					printHelp("stop")
					return false
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return false
				}

				for _, a := range args[1:] {
					err := s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_Stop_{Stop: &landscapeapi.Command_Stop{Id: a}}})
					if err != nil {
						log.Printf("error: %v\n", err)
					}
				}
				return false
			},
			usage: "stop HOST_UID INSTANCES...",
			help:  "Stops the specified instance(s) at the specified host",
		},
		"install": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) < 2 {
					fmt.Println("Wrong usage")
					printHelp("install")
					return false
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return false
				}

				for _, a := range args[1:] {
					err := s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_Install_{Install: &landscapeapi.Command_Install{Id: a}}})
					if err != nil {
						log.Printf("error: %v\n", err)
					}
				}
				return false
			},
			usage: "install HOST_UID INSTANCES...",
			help:  "Installs the specified instance(s) at the specified host",
		},
		"uninstall": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) < 2 {
					fmt.Println("Wrong usage")
					printHelp("uninstall")
					return false
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return false
				}

				for _, a := range args[1:] {
					err := s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_Uninstall_{Uninstall: &landscapeapi.Command_Uninstall{Id: a}}})
					if err != nil {
						log.Printf("error: %v\n", err)
					}
				}
				return false
			},
			usage: "uninstall HOST_UID INSTANCES...",
			help:  "Uninstalls the specified instance(s) at the specified host",
		},
		"set-default": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) != 2 {
					fmt.Println("Wrong usage")
					printHelp("set-default")
					return false
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return false
				}

				err = s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_SetDefault_{SetDefault: &landscapeapi.Command_SetDefault{Id: args[1]}}})
				if err != nil {
					log.Printf("error: %v\n", err)
				}
				return false
			},
			usage: "set-default HOST_UID INSTANCE",
			help:  "Sets the specified instance as default at the specified host",
		},
		"shutdown": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) != 1 {
					fmt.Println("Wrong usage")
					printHelp("shutdown")
					return false
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return false
				}

				err = s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_ShutdownHost_{ShutdownHost: &landscapeapi.Command_ShutdownHost{}}})
				if err != nil {
					log.Printf("error: %v\n", err)
				}
				return false
			},
			usage: "shutdown HOST_UID INSTANCE",
			help:  "Shuts down WSL at the specified host",
		},
		"hosts": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) != 0 {
					fmt.Println("Wrong usage")
					printHelp("hosts")
					return false
				}

				hosts := s.Hosts()

				var uids []string
				for uid := range hosts {
					uids = append(uids, uid)
				}

				sort.Strings(uids)

				for _, uid := range uids {
					connected := "disconnected"
					if s.IsConnected(uid) {
						connected = "connected"
					}
					fmt.Printf("%s %q %s\n", uid, hosts[uid].Hostname, connected)
				}

				return false
			},
			usage: "hosts",
			help:  "Prints a list of all hosts and their UID and status",
		},
		"wait": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) > 1 {
					fmt.Println("Wrong usage")
					return false
				}

				maxT := time.Minute
				if len(args) == 1 {
					t, err := strconv.Atoi(args[0])
					if err != nil {
						fmt.Println("could not parse MAX_TIME")
						return false
					}
					maxT = time.Duration(t) * time.Second
				}

				tk := time.NewTicker(100 * time.Millisecond)
				tm := time.NewTimer(maxT)

				defer tk.Stop()
				defer tm.Stop()

				originalLen := len(s.MessageLog())
				for {
					select {
					case <-tm.C:
						fmt.Println("timeout")
						return false
					case <-tk.C:
					}

					if len(s.MessageLog()) != originalLen {
						return false
					}
				}
			},
			usage: "wait [MAX_TIME]",
			help:  "waits until the next recv, or until MAX_TIME seconds have elapsed (default 60)",
		},
		"disconnect": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
				if len(args) != 1 {
					fmt.Println("Wrong usage")
					return false
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return false
				}

				if err := s.Disconnect(uid); err != nil {
					fmt.Printf("error: %v", err)
				}

				return false
			},
			usage: "disconnect HOST_UID",
			help:  "stops the connection to the specified host",
		},
	}

	// help must be initialized right after to avoid self-reference
	commands["help"] = command{
		callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) bool {
			if len(args) == 1 {
				printHelp(args[0])
				return false
			}

			if len(args) > 1 {
				fmt.Println("Wrong usage")
				printHelp("help")
				return false
			}

			var verbs []string
			for verb := range commands {
				verbs = append(verbs, verb)
			}

			sort.Strings(verbs)

			for _, verb := range verbs {
				printHelp(verb)
			}
			return false
		},
		usage: "help [COMMAND]",
		help:  "prints the help message. If a command is specified, only that command's help is printed",
	}
}

func printHelp(verb string) {
	cmd, ok := commands[verb]
	if !ok {
		fmt.Printf("Verb %q not found\n", verb)
	}

	if cmd.usage != "" {
		fmt.Printf("* %s\n", cmd.usage)
	} else {
		fmt.Printf("* %s\n", verb)
	}
	if cmd.help != "" {
		fmt.Printf("%s\n", cmd.help)
	}
	fmt.Println()
}

// uidRef converts $n into the nth host agent UID (lexicographical order). Zero-indexed.
func uidRef(s *landscapemockservice.Service, uid string) (string, error) {
	const prefix = "@"

	if !strings.HasPrefix(uid, prefix) {
		return uid, nil
	}

	ref, err := strconv.Atoi(uid[len(prefix):])
	if err != nil {
		return uid, fmt.Errorf("could not parse reference %q: format is %sNUMBER", uid, prefix)
	}

	var uids []string
	for uid := range s.Hosts() {
		uids = append(uids, uid)
	}

	if len(uids) <= ref {
		return uid, fmt.Errorf("index overflow: reference %q must be less than host count (%d)", uid, len(uids))
	}

	return uids[ref], nil
}
