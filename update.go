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
	kindnames := []string{"app.create", "app.update", "app.delete", "pool.create", "pool.update", "pool.delete", "node.create"}
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
	poolOperations := make(map[string]operation)

	for name, evs := range group["pool"] {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		op := NewOperation(evs)
		op.target = &poolOperation{poolName: name}
		operations = append(operations, op)
		poolOperations[name] = op
	}

	for _, evs := range group["node"] {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		poolName := evs[0].PoolName()
		op, ok := poolOperations[poolName]
		if ok {
			op.action = "UPDATE"
		} else {
			op = NewOperation(evs)
			op.target = &poolOperation{poolName: poolName}
			operations = append(operations, op)
			poolOperations[poolName] = op
		}
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
		op := NewOperation(evs)
		op.target = &appOperation{appName: name}
		operations = append(operations, op)
	}

	postUpdates(operations)
}

func groupByTarget(events []event) map[string]groupedEvents {
	results := map[string]groupedEvents{
		"app":  groupedEvents{},
		"pool": groupedEvents{},
		"node": groupedEvents{},
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
