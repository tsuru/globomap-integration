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
	action  string // create, update, delete
	appName string
	events  []event
}

func main() {
	tsuru := &tsuruClient{
		Hostname: os.Getenv("TSURU_HOSTNAME"),
		Token:    os.Getenv("TSURU_TOKEN"),
	}
	startTime := time.Now().Add(-24 * time.Hour)
	kindnames := []string{"app.create", "app.update", "app.delete"}
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
	groupedEvents := groupByApp(events)
	operations := []operation{}
	for appName, evs := range groupedEvents {
		if len(evs) == 1 {
			action := strings.TrimPrefix(evs[0].Kind.Name, "app.")
			op := operation{
				appName: appName,
				action:  action,
				events:  evs,
			}
			operations = append(operations, op)
			continue
		}

		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})

		action := strings.TrimPrefix(evs[len(evs)-1].Kind.Name, "app.")
		op := operation{
			appName: appName,
			action:  action,
			events:  evs,
		}
		operations = append(operations, op)
	}

	postUpdates(operations)
}

func groupByApp(events []event) map[string][]event {
	result := make(map[string][]event)
	for _, ev := range events {
		appName := ev.Target.Value
		if _, ok := result[appName]; !ok {
			result[appName] = []event{ev}
		} else {
			result[appName] = append(result[appName], ev)
		}
	}

	return result
}

func postUpdates(operations []operation) {
	globomap := globomapClient{
		Hostname: os.Getenv("GLOBOMAP_HOSTNAME"),
	}
	globomap.Create(operations)
}
