package main

import (
	"flag"
)

type options struct {
	logLevel int
}

func parseArgs() (*options, error) {
	logLevel := flag.Int("log-level", 1, "Level of messages to log, (-1)-3")
	flag.Parse()

	opts := &options{
		logLevel: *logLevel,
	}

	return opts, nil
}
