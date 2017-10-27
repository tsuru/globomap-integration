// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"strings"
)

type update struct{}

func (u *update) Run() {
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
		op := operation{events: evs}
		if parts[0] == "app" {
			op.target = &appOperation{appName: name}
		} else {
			op.target = &poolOperation{poolName: name}
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
