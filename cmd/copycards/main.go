package main

import (
	"flag"
	"fmt"
	"os"

	"copycards/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	exitCode := 0

	switch cmd {
	case "orgs":
		err = handleOrgs(args)
	case "boards":
		err = handleBoards(args)
	case "tickets":
		err = handleTickets(args)
	case "ticket":
		err = handleTicket(args)
	case "diff":
		err = handleDiff(args)
	case "mapping":
		err = handleMapping(args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	os.Exit(exitCode)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `copycards - Copy Flowboards tickets between organizations

Usage:
  copycards orgs list
  copycards orgs verify <profile>
  copycards boards list --from <profile>
  copycards board verify --from <src> --to <dst> --src-board <id> --dst-board <id>
  copycards tickets copy --from <src> --to <dst> --src-board <id> --dst-board <id> [--dry-run] [--include-attachments] [--include-comments] [--concurrency N]
  copycards ticket copy <id> --from <src> --to <dst> --dst-board <id> [--with-children] [--include-attachments] [--include-comments] [--dry-run]
  copycards diff --from <src> --to <dst> --src-board <id> --dst-board <id>
  copycards mapping show [--from <src> --to <dst> --src-board <id>]
  copycards mapping reset [--from <src> --to <dst> --src-board <id>]

`)
}

func handleOrgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("orgs: subcommand required (list, verify)")
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "list":
		return cli.ListOrgs()

	case "verify":
		if len(rest) == 0 {
			return fmt.Errorf("orgs verify: profile name required")
		}
		return cli.VerifyOrgAuth(rest[0])

	default:
		return fmt.Errorf("unknown orgs subcommand: %s", subcmd)
	}
}

func handleBoards(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("boards: subcommand required (list, verify)")
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "list":
		fs := flag.NewFlagSet("boards list", flag.ContinueOnError)
		from := fs.String("from", "", "source profile")
		if err := fs.Parse(rest); err != nil {
			return err
		}

		if *from == "" {
			return fmt.Errorf("boards list: --from <profile> required")
		}

		return cli.ListBoards(*from)

	case "verify":
		fs := flag.NewFlagSet("board verify", flag.ContinueOnError)
		from := fs.String("from", "", "source profile")
		to := fs.String("to", "", "destination profile")
		srcBoard := fs.String("src-board", "", "source board ID")
		dstBoard := fs.String("dst-board", "", "destination board ID")

		if err := fs.Parse(rest); err != nil {
			return err
		}

		if *from == "" || *to == "" || *srcBoard == "" || *dstBoard == "" {
			return fmt.Errorf("board verify: --from, --to, --src-board, --dst-board required")
		}

		return cli.VerifyBoards(*from, *to, *srcBoard, *dstBoard)

	default:
		return fmt.Errorf("unknown boards subcommand: %s", subcmd)
	}
}

func handleTickets(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("tickets: subcommand required (copy)")
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "copy":
		fs := flag.NewFlagSet("tickets copy", flag.ContinueOnError)
		from := fs.String("from", "", "source profile")
		to := fs.String("to", "", "destination profile")
		srcBoard := fs.String("src-board", "", "source board ID")
		dstBoard := fs.String("dst-board", "", "destination board ID")
		dryRun := fs.Bool("dry-run", false, "preview changes without applying")
		incAttach := fs.Bool("include-attachments", false, "copy attachments")
		incComments := fs.Bool("include-comments", false, "copy comments")
		concurrency := fs.Int("concurrency", 4, "number of concurrent requests (1-500)")

		if err := fs.Parse(rest); err != nil {
			return err
		}

		if *from == "" || *to == "" || *srcBoard == "" || *dstBoard == "" {
			return fmt.Errorf("tickets copy: --from, --to, --src-board, --dst-board required")
		}

		opts := cli.CopyTicketsOptions{
			DryRun:             *dryRun,
			IncludeAttachments: *incAttach,
			IncludeComments:    *incComments,
			Concurrency:        *concurrency,
		}

		return cli.CopyTickets(*from, *to, *srcBoard, *dstBoard, opts)

	default:
		return fmt.Errorf("unknown tickets subcommand: %s", subcmd)
	}
}

func handleTicket(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("ticket: subcommand required (copy)")
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "copy":
		fs := flag.NewFlagSet("ticket copy", flag.ContinueOnError)
		from := fs.String("from", "", "source profile")
		to := fs.String("to", "", "destination profile")
		dstBoard := fs.String("dst-board", "", "destination board ID")
		withChildren := fs.Bool("with-children", false, "copy child tickets")
		incAttach := fs.Bool("include-attachments", false, "copy attachments")
		incComments := fs.Bool("include-comments", false, "copy comments")
		dryRun := fs.Bool("dry-run", false, "preview changes without applying")

		if err := fs.Parse(rest); err != nil {
			return err
		}

		if len(fs.Args()) == 0 {
			return fmt.Errorf("ticket copy: ticket ID required as first argument")
		}

		ticketID := fs.Args()[0]

		if *from == "" || *to == "" || *dstBoard == "" {
			return fmt.Errorf("ticket copy: --from, --to, --dst-board required")
		}

		opts := struct {
			WithChildren       bool
			IncludeAttachments bool
			IncludeComments    bool
			DryRun             bool
		}{
			WithChildren:       *withChildren,
			IncludeAttachments: *incAttach,
			IncludeComments:    *incComments,
			DryRun:             *dryRun,
		}

		return cli.CopyTicket(*from, *to, ticketID, *dstBoard, opts)

	default:
		return fmt.Errorf("unknown ticket subcommand: %s", subcmd)
	}
}

func handleDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	from := fs.String("from", "", "source profile")
	to := fs.String("to", "", "destination profile")
	srcBoard := fs.String("src-board", "", "source board ID")
	dstBoard := fs.String("dst-board", "", "destination board ID")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *from == "" || *to == "" || *srcBoard == "" || *dstBoard == "" {
		return fmt.Errorf("diff: --from, --to, --src-board, --dst-board required")
	}

	return cli.DiffBoards(*from, *to, *srcBoard, *dstBoard)
}

func handleMapping(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("mapping: subcommand required (show, reset)")
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "show":
		fs := flag.NewFlagSet("mapping show", flag.ContinueOnError)
		from := fs.String("from", "", "source profile")
		to := fs.String("to", "", "destination profile")
		srcBoard := fs.String("src-board", "", "source board ID")

		if err := fs.Parse(rest); err != nil {
			return err
		}

		return cli.ShowMapping(*from, *to, *srcBoard)

	case "reset":
		fs := flag.NewFlagSet("mapping reset", flag.ContinueOnError)
		from := fs.String("from", "", "source profile")
		to := fs.String("to", "", "destination profile")
		srcBoard := fs.String("src-board", "", "source board ID")

		if err := fs.Parse(rest); err != nil {
			return err
		}

		return cli.ResetMapping(*from, *to, *srcBoard)

	default:
		return fmt.Errorf("unknown mapping subcommand: %s", subcmd)
	}
}
