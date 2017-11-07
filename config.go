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
	tsuruHostname          string
	tsuruToken             string
	globomapApiHostname    string
	globomapLoaderHostname string
	startTime              *time.Time
	repeat                 *time.Duration
}

type flags struct {
	fs        *gnuflag.FlagSet
	dry       bool
	startTime string
	load      bool
	repeat    string
}

func NewConfig() configParams {
	return configParams{
		tsuruHostname:          os.Getenv("TSURU_HOSTNAME"),
		tsuruToken:             os.Getenv("TSURU_TOKEN"),
		globomapApiHostname:    os.Getenv("GLOBOMAP_API_HOSTNAME"),
		globomapLoaderHostname: os.Getenv("GLOBOMAP_LOADER_HOSTNAME"),
	}
}

func (c *configParams) ProcessArguments(args []string) error {
	flags := flags{fs: gnuflag.NewFlagSet("", gnuflag.ExitOnError)}
	flags.fs.BoolVar(&flags.dry, "dry", false, "enable dry mode")
	flags.fs.BoolVar(&flags.dry, "d", false, "enable dry mode")
	flags.fs.StringVar(&flags.startTime, "start", "", "start time")
	flags.fs.StringVar(&flags.startTime, "s", "", "start time")
	flags.fs.BoolVar(&flags.load, "load", false, "load all data")
	flags.fs.BoolVar(&flags.load, "l", false, "load all data")
	flags.fs.StringVar(&flags.repeat, "repeat", "", "repeat frequency")
	flags.fs.StringVar(&flags.repeat, "r", "", "repeat frequency")
	err := flags.fs.Parse(true, args)
	if err != nil {
		return err
	}

	if flags.load && flags.startTime != "" {
		return errors.New("Load mode doesn't support --start flag")
	}
	if flags.load && flags.repeat != "" {
		return errors.New("Load mode doesn't support --repeat flag")
	}
	if flags.startTime != "" && flags.repeat != "" {
		return errors.New("--start and --repeat flags can't be used together")
	}

	c.dry = flags.dry
	if flags.load {
		env.cmd = &loadCmd{}
	} else {
		env.cmd = &updateCmd{}
		c.startTime, err = c.parseTime(flags.startTime)
		if err != nil {
			return err
		}
		if c.startTime == nil {
			t := time.Now().Add(-24 * time.Hour)
			c.startTime = &t
		}
		c.repeat, err = c.parseTimeDuration(flags.repeat)
		if err != nil {
			return err
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

func (c *configParams) parseTime(timeStr string) (*time.Time, error) {
	duration, err := c.parseTimeDuration(timeStr)
	if duration == nil || err != nil {
		return nil, err
	}
	t := time.Now().Add(time.Duration(-1) * *duration)
	return &t, nil
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
