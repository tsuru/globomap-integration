// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/tsuru/gnuflag"
)

type configParams struct {
	dry                    bool
	verbose                bool
	tsuruHostname          string
	tsuruToken             string
	globomapApiHostname    string
	globomapLoaderHostname string
	start                  *time.Duration
	repeat                 *time.Duration
	retrySleepTime         time.Duration
	maxRetries             int
	sleepTimeBetweenChunks time.Duration
}

type flags struct {
	fs      *gnuflag.FlagSet
	dry     bool
	verbose bool
	start   string
	load    bool
	repeat  string
}

func NewConfig() configParams {
	config := configParams{
		tsuruHostname:          os.Getenv("TSURU_HOSTNAME"),
		tsuruToken:             os.Getenv("TSURU_TOKEN"),
		globomapApiHostname:    os.Getenv("GLOBOMAP_API_HOSTNAME"),
		globomapLoaderHostname: os.Getenv("GLOBOMAP_LOADER_HOSTNAME"),
		retrySleepTime:         5 * time.Minute,
		maxRetries:             20,
		sleepTimeBetweenChunks: 10 * time.Second,
	}
	config.processRetryArguments()
	return config
}

func (c *configParams) processRetryArguments() {
	retry, err := c.parseTimeDuration(os.Getenv("RETRY_SLEEP_TIME"))
	if retry != nil && err == nil {
		c.retrySleepTime = *retry
	}

	max := os.Getenv("MAX_RETRIES")
	if max != "" {
		maxInt, err := strconv.Atoi(max)
		if err == nil && maxInt > 0 {
			c.maxRetries = maxInt
		}
	}
}

func (c *configParams) ProcessArguments(args []string) error {
	flags := flags{fs: gnuflag.NewFlagSet("", gnuflag.ExitOnError)}
	flags.fs.BoolVar(&flags.dry, "dry", false, "dry mode")
	flags.fs.BoolVar(&flags.dry, "d", false, "dry mode")
	flags.fs.BoolVar(&flags.verbose, "verbose", false, "verbose mode")
	flags.fs.BoolVar(&flags.verbose, "v", false, "verbose mode")
	flags.fs.StringVar(&flags.start, "start", "", "start time")
	flags.fs.StringVar(&flags.start, "s", "", "start time")
	flags.fs.BoolVar(&flags.load, "load", false, "load mode")
	flags.fs.BoolVar(&flags.load, "l", false, "load mode")
	flags.fs.StringVar(&flags.repeat, "repeat", "", "repeat frequency")
	flags.fs.StringVar(&flags.repeat, "r", "", "repeat frequency")
	err := flags.fs.Parse(true, args)
	if err != nil {
		return err
	}

	if flags.load && flags.start != "" {
		return errors.New("Load mode doesn't support --start flag")
	}
	if flags.load && flags.repeat != "" {
		return errors.New("Load mode doesn't support --repeat flag")
	}
	if flags.start != "" && flags.repeat != "" {
		return errors.New("--start and --repeat flags can't be set together")
	}

	c.dry = flags.dry
	c.verbose = flags.verbose
	if flags.load {
		env.cmd = &loadCmd{}
	} else {
		env.cmd = &updateCmd{}
		c.repeat, err = c.parseTimeDuration(flags.repeat)
		if err != nil {
			return err
		}
		c.start, err = c.parseTimeDuration(flags.start)
		if err != nil {
			return err
		}
		if c.start == nil {
			var d time.Duration
			if c.repeat != nil {
				// Add a 10 minute margin to start time
				margin := time.Duration(10 * time.Minute)
				if margin > *c.repeat {
					margin = *c.repeat
				}
				d = margin + *c.repeat
			} else {
				d = time.Duration(24 * time.Hour)
			}
			c.start = &d
		}
	}

	if c.tsuruHostname == "" {
		return errors.New("TSURU_HOSTNAME is required")
	}
	if c.tsuruToken == "" {
		return errors.New("TSURU_TOKEN is required")
	}
	if c.globomapApiHostname == "" {
		return errors.New("GLOBOMAP_API_HOSTNAME is required")
	}
	if !c.dry && c.globomapLoaderHostname == "" {
		return errors.New("GLOBOMAP_LOADER_HOSTNAME is required")
	}
	return nil
}

func (c *configParams) parseTimeDuration(timeStr string) (*time.Duration, error) {
	if timeStr == "" {
		return nil, nil
	}
	r, err := regexp.Compile(`^(\d+) ?(\w)$`)
	if err != nil {
		return nil, errors.New("Invalid start argument")
	}
	matches := r.FindStringSubmatch(timeStr)
	if len(matches) != 3 {
		return nil, fmt.Errorf("Invalid start argument: %s", timeStr)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("Invalid start argument: %s is not a valid number", matches[1])
	}
	unit := matches[2]

	var d time.Duration
	switch unit {
	case "d":
		d = time.Duration(24*value) * time.Hour
	case "h":
		d = time.Duration(value) * time.Hour
	case "m":
		d = time.Duration(value) * time.Minute
	default:
		return nil, fmt.Errorf("Invalid start argument: %s is not a valid unit", unit)
	}
	return &d, nil
}
