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
)

type configParams struct {
	flags         flags
	dry           bool
	tsuruHostname string
	tsuruToken    string
	startTime     time.Time
}

func (c *configParams) processArguments(args []string) error {
	config.flags.fs.BoolVar(&config.flags.dry, "dry", false, "enable dry mode")
	config.flags.fs.BoolVar(&config.flags.dry, "d", false, "enable dry mode")
	config.flags.fs.StringVar(&config.flags.startTime, "start", "1h", "start time")
	config.flags.fs.StringVar(&config.flags.startTime, "s", "1h", "start time")
	err := config.flags.fs.Parse(true, args)
	if err != nil {
		return err
	}
	config.dry = config.flags.dry

	err = config.parseStartTime()
	if err != nil {
		return err
	}
	if c.tsuruHostname == "" {
		return errors.New("TSURU_HOSTNAME is required")
	}
	if c.tsuruToken == "" {
		return errors.New("TSURU_TOKEN is required")
	}
	return nil
}

func (c *configParams) parseStartTime() error {
	if config.flags.startTime == "" {
		return nil
	}
	r, err := regexp.Compile(`^(\d+) ?(\w)$`)
	if err != nil {
		return errors.New("Invalid start argument")
	}
	matches := r.FindStringSubmatch(config.flags.startTime)
	if len(matches) != 3 {
		return fmt.Errorf("Invalid start argument: %s", config.flags.startTime)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("Invalid start argument: %s is not a valid number", matches[1])
	}
	unit := matches[2]
	switch unit {
	case "d":
		config.startTime = time.Now().Add(time.Duration(-24*value) * time.Hour)
	case "h":
		config.startTime = time.Now().Add(time.Duration(-1*value) * time.Hour)
	case "m":
		config.startTime = time.Now().Add(time.Duration(-1*value) * time.Minute)
	default:
		return fmt.Errorf("Invalid start argument: %s is not a valid unit", unit)

	}
	return nil
}
