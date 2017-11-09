// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"time"
)

type updateCmd struct{}

type groupedEvents map[string][]event

func (u *updateCmd) Run() {
	kindnames := []string{
		"app.create", "app.update", "app.delete",
		"pool.create", "pool.update", "pool.delete",
		"node.create", "node.delete",
		"healer",
	}
	since := time.Now().Add(-1 * *env.config.start)
	f := eventFilter{
		Kindnames: kindnames,
		Since:     &since,
	}
	events, err := env.tsuru.EventList(f)
	if err != nil {
		return
	}

	processEvents(events)
}

func processEvents(events []event) {
	group := groupByTarget(events)
	operations := []operation{}

	for name, evs := range group["pool"] {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		endTime := evs[len(evs)-1].EndTime
		lastStatus := eventStatus(evs[len(evs)-1])
		op := &poolOperation{
			action:   lastStatus,
			time:     endTime,
			poolName: name,
		}
		operations = append(operations, op)
	}

	for addr, evs := range group["node"] {
		endTime := evs[len(evs)-1].EndTime
		lastStatus := eventStatus(evs[len(evs)-1])
		op := &nodeOperation{
			action:   lastStatus,
			time:     endTime,
			nodeAddr: addr,
		}
		operations = append(operations, op)
	}

	for addr, evs := range group["healer"] {
		endTime := evs[len(evs)-1].EndTime
		removedNodeOp := &nodeOperation{
			action:   "DELETE",
			time:     endTime,
			nodeAddr: addr,
		}
		operations = append(operations, removedNodeOp)

		var data map[string]string
		err := evs[0].EndData(&data)
		if err != nil {
			continue
		}
		addedNodeOp := &nodeOperation{
			action:   "UPDATE",
			time:     endTime,
			nodeAddr: data["_id"],
		}
		operations = append(operations, addedNodeOp)
	}

	if len(operations) > 0 {
		var err error
		env.pools, err = env.tsuru.PoolList()
		if err != nil {
			fmt.Println("Error retrieving pool list: ", err)
			return
		}
	}

	for name, evs := range group["app"] {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		endTime := evs[len(evs)-1].EndTime
		lastStatus := eventStatus(evs[len(evs)-1])
		op := &appOperation{
			action:  lastStatus,
			time:    endTime,
			appName: name,
		}
		operations = append(operations, op)

		appPoolOp := &appPoolOperation{
			action:  lastStatus,
			time:    endTime,
			appName: name,
		}
		operations = append(operations, appPoolOp)
	}

	postUpdates(operations)
}

func groupByTarget(events []event) map[string]groupedEvents {
	results := map[string]groupedEvents{
		"app":    groupedEvents{},
		"pool":   groupedEvents{},
		"node":   groupedEvents{},
		"healer": groupedEvents{},
	}

	for _, ev := range events {
		if ev.Failed() {
			continue
		}

		name := ev.Target.Value
		evType := ev.Target.Type
		if _, ok := results[evType][name]; !ok {
			results[evType][name] = []event{ev}
		} else {
			results[evType][name] = append(results[evType][name], ev)
		}
	}

	return results
}
