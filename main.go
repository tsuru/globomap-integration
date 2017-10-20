// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tsuru/gnuflag"
)

type operation struct {
	action     string // create, update, delete
	name       string
	collection string
	docType    string
	events     []event
	app        *app
}

type configParams struct {
	flags         flags
	dry           bool
	tsuruHostname string
	tsuruToken    string
	startTime     time.Time
}

type flags struct {
	fs        *gnuflag.FlagSet
	dry       bool
	startTime string
}

var config configParams
var tsuru *tsuruClient

func (op *operation) Time() time.Time {
	if len(op.events) > 0 {
		return op.events[len(op.events)-1].EndTime
	}
	return time.Now()
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

func setup(args []string) {
	config = configParams{
		flags:         flags{fs: gnuflag.NewFlagSet("", gnuflag.ExitOnError)},
		tsuruHostname: os.Getenv("TSURU_HOSTNAME"),
		tsuruToken:    os.Getenv("TSURU_TOKEN"),
		startTime:     time.Now().Add(-24 * time.Hour),
	}
	err := config.processArguments(args)
	if err != nil {
		panic(err)
	}
	tsuru = &tsuruClient{
		Hostname: config.tsuruHostname,
		Token:    config.tsuruToken,
	}
}

func main() {
	setup(os.Args[1:])
	kindnames := []string{"app.create", "app.update", "app.delete", "pool.create", "pool.update", "pool.delete"}
	events := make(chan []event, len(kindnames))
	for _, kindname := range kindnames {
		go func(kindname string) {
			f := eventFilter{
				Kindname: kindname,
				Since:    &config.startTime,
			}
			ev, err := tsuru.EventList(f)
			if err != nil {
				events <- nil
				return
			}
			events <- ev
		}(kindname)
	}

	eventList := []event{}
	for i := 0; i < len(kindnames); i++ {
		eventList = append(eventList, <-events...)
	}

	processEvents(eventList)
}

func processEvents(events []event) {
	groupedEvents := groupByTarget(events)
	operations := []operation{}
	for name, evs := range groupedEvents {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		parts := strings.Split(evs[len(evs)-1].Kind.Name, ".")
		collection := "tsuru_" + parts[0]
		action := parts[1]
		op := operation{
			name:       name,
			action:     action,
			collection: collection,
			docType:    "collections",
			events:     evs,
		}
		operations = append(operations, op)

		if collection == "tsuru_app" {
			app, err := tsuru.AppInfo(name)
			if err != nil {
				continue
			}
			op := operation{
				action:     "CREATE",
				collection: "tsuru_pool_app",
				docType:    "edges",
				events:     evs,
				app:        app,
			}
			operations = append(operations, op)
		}
	}

	postUpdates(operations)
}

func groupByTarget(events []event) map[string][]event {
	result := make(map[string][]event)
	for _, ev := range events {
		name := ev.Target.Value
		if _, ok := result[name]; !ok {
			result[name] = []event{ev}
		} else {
			result[name] = append(result[name], ev)
		}
	}

	return result
}

func postUpdates(operations []operation) {
	globomap := globomapClient{
		Hostname: os.Getenv("GLOBOMAP_HOSTNAME"),
	}
	globomap.Post(operations)
}
