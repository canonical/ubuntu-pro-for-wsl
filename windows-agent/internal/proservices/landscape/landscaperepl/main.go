package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape/landscapemockservice"
	"google.golang.org/grpc"
)

var exit func()

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("This program should only have the address to host on as an argument")
	}
	addr := os.Args[1]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exit = cancel
	initCommands()

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp", addr)
	if err != nil {
		log.Fatalf("Can't listen: %v", err)
	}
	defer lis.Close()

	server := grpc.NewServer()
	service := landscapemockservice.New()
	landscapeapi.RegisterLandscapeHostAgentServer(server, service)

	go server.Serve(lis)

	if err := repl(ctx, service); err != nil {
		log.Fatalf("%v", err)
	}
}

// REPL: Read, Execute, Print, Loop
func repl(ctx context.Context, s *landscapemockservice.Service) error {
	sc := bufio.NewScanner(os.Stdin)

	prefix := "$ "

	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// data is from pipe
		prefix = ""
	}

	fmt.Printf(prefix)

	// READ
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		// EXECUTE + PRINT
		done := execute(ctx, s, line)
		if done {
			break
		}

		// LOOP
		fmt.Printf(prefix)
	}

	if err := sc.Err(); err != nil {
		return err
	}

	return nil
}

func execute(ctx context.Context, s *landscapemockservice.Service, command string) (exit bool) {
	fields := strings.Fields(command)

	verb := fields[0]
	args := fields[1:]

	cmd, ok := commands[verb]
	if !ok {
		fmt.Printf("unknown verb %q. use 'help' to see available commands\n", verb)
		return false
	}

	return cmd.callback(ctx, s, args...)
}
