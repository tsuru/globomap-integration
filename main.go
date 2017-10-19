// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"sort"
	"strings"
	"time"
)

type operation struct {
	action     string // create, update, delete
	name       string
	collection string
	docType    string
	events     []event
}

var dry bool = false

func (op *operation) Time() time.Time {
	if len(op.events) > 0 {
		return op.events[len(op.events)-1].EndTime
	}
	return time.Now()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "dry" {
		dry = true
	}
	tsuru := &tsuruClient{
		Hostname: os.Getenv("TSURU_HOSTNAME"),
		Token:    os.Getenv("TSURU_TOKEN"),
	}
	startTime := time.Now().Add(-24 * time.Hour)
	kindnames := []string{"app.create", "app.update", "app.delete", "pool.create", "pool.update", "pool.delete"}
	events := make(chan []event, len(kindnames))
	for _, kindname := range kindnames {
		go func(kindname string) {
			f := eventFilter{
				Kindname: kindname,
				Since:    &startTime,
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
			op := operation{
				action:     "CREATE",
				collection: "tsuru_pool_app",
				docType:    "edges",
				events:     evs,
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
