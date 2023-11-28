package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/landscape/landscapemockservice"
)

type command struct {
	callback func(ctx context.Context, s *landscapemockservice.Service, args ...string) error
	usage    string
	help     string
}

var commands map[string]command

func populateCommands() {
	commands = map[string]command{
		"exit": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				return exitError{}
			},
			help: "Exits the program.",
		},
		"status": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) != 1 {
					return wrongUsageError{}
				}

				hosts := s.Hosts()

				uid, err := uidRef(s, args[0])
				if err != nil {
					return err
				}

				host, ok := hosts[uid]
				if !ok {
					return fmt.Errorf("HOST_UID not found")
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

				return nil
			},
			usage: "status HOST_UID",
			help:  "Shows the status of the specified host.",
		},
		"journal": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				for _, line := range s.MessageLog() {
					var instances []string
					for _, inst := range line.Instances {
						instances = append(instances, inst.ID)
					}

					fmt.Printf("UID: %s, Account: %q, Key: %q, Token: %q, Hostname: %q, Instances: %q\n",
						line.UID, line.AccountName, line.RegistrationKey, common.Obfuscate(line.Token), line.Hostname, strings.Join(instances, ", "))
				}
				return nil
			},
			help: "Prints the log.",
		},
		"start": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) != 2 {
					return wrongUsageError{}
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					return err
				}

				err = s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_Start_{Start: &landscapeapi.Command_Start{Id: args[1]}}})
				if err != nil {
					return err
				}

				return nil
			},
			usage: "start HOST_UID INSTANCE",
			help:  "Starts the specified instance at the specified host.",
		},
		"stop": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) != 2 {
					return wrongUsageError{}
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					return err
				}

				err = s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_Stop_{Stop: &landscapeapi.Command_Stop{Id: args[1]}}})
				if err != nil {
					return err
				}

				return nil
			},
			usage: "stop HOST_UID INSTANCE",
			help:  "Stops the specified instance at the specified host.",
		},
		"install": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) != 2 {
					return wrongUsageError{}
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					return err
				}

				err = s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_Install_{Install: &landscapeapi.Command_Install{Id: args[1]}}})
				if err != nil {
					log.Printf("error: %v\n", err)
				}

				return nil
			},
			usage: "install HOST_UID INSTANCE",
			help:  "Installs the specified instance at the specified host.",
		},
		"uninstall": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) != 2 {
					return wrongUsageError{}
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					return err
				}

				err = s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_Uninstall_{Uninstall: &landscapeapi.Command_Uninstall{Id: args[1]}}})
				if err != nil {
					return err
				}

				return nil
			},
			usage: "uninstall HOST_UID INSTANCES",
			help:  "Uninstalls the specified instance at the specified host.",
		},
		"set-default": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) < 2 {
					return wrongUsageError{}
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					return err
				}

				err = s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_SetDefault_{SetDefault: &landscapeapi.Command_SetDefault{Id: args[1]}}})
				if err != nil {
					return err
				}

				return nil
			},
			usage: "set-default HOST_UID INSTANCE",
			help:  "Sets the specified instance as default at the specified host.",
		},
		"shutdown": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) != 1 {
					return wrongUsageError{}
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					return err
				}

				err = s.SendCommand(ctx, uid, &landscapeapi.Command{Cmd: &landscapeapi.Command_ShutdownHost_{ShutdownHost: &landscapeapi.Command_ShutdownHost{}}})
				if err != nil {
					return err
				}

				return nil
			},
			usage: "shutdown HOST_UID INSTANCE",
			help:  "Shuts down WSL at the specified host.",
		},
		"hosts": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) != 0 {
					return wrongUsageError{}
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

				return nil
			},
			usage: "hosts",
			help:  "Prints a list of all hosts and their UID and status.",
		},
		"wait": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) > 1 {
					return wrongUsageError{}
				}

				maxT := time.Minute
				if len(args) == 1 {
					t, err := strconv.Atoi(args[0])
					if err != nil {
						return fmt.Errorf("could not parse MAX_TIME")
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
						return fmt.Errorf("timeout")
					case <-tk.C:
					}

					if len(s.MessageLog()) != originalLen {
						return nil
					}
				}
			},
			usage: "wait [MAX_TIME]",
			help:  "Waits until the next recv, or until MAX_TIME seconds have elapsed (default 60).",
		},
		"disconnect": {
			callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
				if len(args) != 1 {
					return wrongUsageError{}
				}

				uid, err := uidRef(s, args[0])
				if err != nil {
					return err
				}

				if err := s.Disconnect(uid); err != nil {
					return err
				}

				return nil
			},
			usage: "disconnect HOST_UID",
			help:  "Stops the connection to the specified host.",
		},
	}

	// help must be initialized right after to avoid self-reference
	commands["help"] = command{
		callback: func(ctx context.Context, s *landscapemockservice.Service, args ...string) error {
			switch len(args) {
			case 0:
			case 1:
				showHelp(os.Stderr, args[0])
				return nil
			default:
				return wrongUsageError{}
			}

			var verbs []string
			for verb := range commands {
				//            ~~~~~~~~
				// Self-reference here
				verbs = append(verbs, verb)
			}

			sort.Strings(verbs)

			for _, verb := range verbs {
				showHelp(os.Stdout, verb)
				fmt.Println()
			}

			return nil
		},
		usage: "help [COMMAND]",
		help:  "Prints the help message. If a command is specified, only that command's help is printed.",
	}
}

func showHelp(w io.Writer, verb string) {
	cmd, ok := commands[verb]
	if !ok {
		if _, err := fmt.Fprintf(w, "Verb %q not found\n", verb); err != nil {
			log.Fatalf("could not write: %v", err)
		}

		return
	}

	if cmd.usage != "" {
		if _, err := fmt.Fprintf(w, "* %s\n", cmd.usage); err != nil {
			log.Fatalf("could not write: %v", err)
		}
	} else {
		if _, err := fmt.Fprintf(w, "* %s\n", verb); err != nil {
			log.Fatalf("could not write: %v", err)
		}
	}
	if cmd.help != "" {
		if _, err := fmt.Fprintf(w, "%s\n", cmd.help); err != nil {
			log.Fatalf("could not write: %v", err)
		}
	}
}

// uidRef converts @n into the nth host agent UID (lexicographical order). Zero-indexed.
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
