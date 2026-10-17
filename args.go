package main

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
)

// Args holds the parsed command-line options.
type Args struct {
	Directory string
	Depth     int
	Date      string
	From      string
	To        string
	Authors   []string
	Verbose   bool
	Config    string
	Version   bool
}

func parseArgs(argv []string) (*Args, error) {
	args := &Args{}
	fs := flag.NewFlagSet("git-bluff", flag.ContinueOnError)
	fs.SortFlags = false
	fs.SetOutput(os.Stdout)

	fs.StringVarP(&args.Directory, "directory", "d", ".", "Starting directory to scan")
	fs.IntVar(&args.Depth, "depth", 1, "Maximum directory depth to scan")
	fs.StringVar(&args.Date, "date", "", "Filter commits by single date (format: YYYY-MM-DD)")
	fs.StringVar(&args.From, "from", "", "Start date for date range (format: YYYY-MM-DD)")
	fs.StringVar(&args.To, "to", "", "End date for date range (format: YYYY-MM-DD)")
	fs.StringSliceVar(&args.Authors, "author", nil, "Filter commits by author name (comma-separated, repeatable)")
	fs.BoolVarP(&args.Verbose, "verbose", "v", false, "Show repository paths in output")
	fs.StringVar(&args.Config, "config", "", "Path to YAML configuration file")
	fs.BoolVarP(&args.Version, "version", "V", false, "Print version")

	if err := fs.Parse(argv); err != nil {
		if err == flag.ErrHelp {
			return nil, errHelp
		}
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument: %s", fs.Arg(0))
	}
	return args, nil
}

// errHelp signals a help request; usage has already been printed by pflag.
var errHelp = fmt.Errorf("help requested")
