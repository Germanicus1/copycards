package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: copycards <command> [flags]\n")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "orgs":
		handleOrgs(args)
	case "boards":
		handleBoards(args)
	case "tickets":
		handleTickets(args)
	case "ticket":
		handleTicket(args)
	case "diff":
		handleDiff(args)
	case "mapping":
		handleMapping(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func handleOrgs(args []string) {
	fs := flag.NewFlagSet("orgs", flag.ExitOnError)
	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
	}
	switch subcmd {
	case "list":
		fmt.Println("TODO: orgs list")
	default:
		fs.Usage()
	}
}

func handleBoards(args []string) {
	fmt.Println("TODO: boards command")
}

func handleTickets(args []string) {
	fmt.Println("TODO: tickets command")
}

func handleTicket(args []string) {
	fmt.Println("TODO: ticket command")
}

func handleDiff(args []string) {
	fmt.Println("TODO: diff command")
}

func handleMapping(args []string) {
	fmt.Println("TODO: mapping command")
}
