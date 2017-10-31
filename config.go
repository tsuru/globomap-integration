// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/tsuru/gnuflag"
)

type configParams struct {
	dry                    bool
	tsuruHostname          string
	tsuruToken             string
	globomapApiHostname    string
	globomapLoaderHostname string
	startTime              time.Time
}

type flags struct {
	fs        *gnuflag.FlagSet
	dry       bool
	startTime string
	load      bool
}

func (c *configParams) processArguments(args []string) error {
	flags := flags{fs: gnuflag.NewFlagSet("", gnuflag.ExitOnError)}
	flags.fs.BoolVar(&flags.dry, "dry", false, "enable dry mode")
	flags.fs.BoolVar(&flags.dry, "d", false, "enable dry mode")
	flags.fs.StringVar(&flags.startTime, "start", "1h", "start time")
	flags.fs.StringVar(&flags.startTime, "s", "1h", "start time")
	flags.fs.BoolVar(&flags.load, "load", false, "load all data")
	flags.fs.BoolVar(&flags.load, "l", false, "load all data")
	err := flags.fs.Parse(true, args)
	if err != nil {
		return err
	}
	env.config.dry = flags.dry
	if flags.load {
		env.cmd = &loadCmd{}
	} else {
		env.cmd = &updateCmd{}
	}

	err = env.config.parseStartTime(flags.startTime)
	if err != nil {
		return err
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

func (c *configParams) parseStartTime(startTime string) error {
	if startTime == "" {
		return nil
	}
	r, err := regexp.Compile(`^(\d+) ?(\w)$`)
	if err != nil {
		return errors.New("Invalid start argument")
	}
	matches := r.FindStringSubmatch(startTime)
	if len(matches) != 3 {
		return fmt.Errorf("Invalid start argument: %s", startTime)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("Invalid start argument: %s is not a valid number", matches[1])
	}
	unit := matches[2]
	switch unit {
	case "d":
		env.config.startTime = time.Now().Add(time.Duration(-24*value) * time.Hour)
	case "h":
		env.config.startTime = time.Now().Add(time.Duration(-1*value) * time.Hour)
	case "m":
		env.config.startTime = time.Now().Add(time.Duration(-1*value) * time.Minute)
	default:
		return fmt.Errorf("Invalid start argument: %s is not a valid unit", unit)

	}
	return nil
}
