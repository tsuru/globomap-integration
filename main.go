// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tsuru/gnuflag"
)

type flags struct {
	fs        *gnuflag.FlagSet
	dry       bool
	startTime string
}

var config configParams
var tsuru *tsuruClient
var pools []pool

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
	var hasPoolEvents bool
	for name, evs := range groupedEvents {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		parts := strings.Split(evs[len(evs)-1].Kind.Name, ".")
		if parts[0] == "pool" {
			hasPoolEvents = true
		}
		collection := "tsuru_" + parts[0]
		op := operation{
			name:       name,
			collection: collection,
			events:     evs,
		}
		operations = append(operations, op)
	}

	if hasPoolEvents {
		var err error
		pools, err = tsuru.PoolList()
		if err != nil {
			fmt.Println("Error retrieving pool list: ", err)
			return
		}
	}

	postUpdates(operations)
}

func groupByTarget(events []event) map[string][]event {
	result := map[string][]event{}
	for _, ev := range events {
		if ev.Failed() {
			continue
		}
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
