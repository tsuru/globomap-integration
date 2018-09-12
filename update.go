// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type updateCmd struct{}

type groupedEvents map[string][]event

func (u *updateCmd) Run() {
	events := fetchEvents()
	if env.config.verbose {
		fmt.Printf("Found %d events\n", len(events))
	}
	processEvents(events)
}

func fetchEvents() []event {
	since := time.Now().Add(-1 * *env.config.start)
	if env.config.verbose {
		fmt.Printf("Fetching events since %s\n", since)
	}

	filters := []eventFilter{
		{Kindnames: []string{
			"app.create", "app.update", "app.delete",
			"pool.create", "pool.update", "pool.delete",
			"node.create", "node.delete",
		}, Since: &since},
		{Kindnames: []string{"healer"}, TargetType: "node", Since: &since},
	}

	eventStream := make(chan []event, len(filters))
	var wg sync.WaitGroup
	wg.Add(len(filters))

	for _, f := range filters {
		go func(f eventFilter) {
			defer wg.Done()
			events, err := env.tsuru.EventList(f)
			if err != nil {
				if env.config.verbose {
					fmt.Printf("Error fetching events: %s\n", err)
				}
			} else {
				eventStream <- events
			}
		}(f)
	}

	go func() {
		defer close(eventStream)
		wg.Wait()
	}()

	events := []event{}
	for evs := range eventStream {
		events = append(events, evs...)
	}
	return events
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
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			poolName: name,
		}
		operations = append(operations, op)
	}

	if len(operations) > 0 {
		var err error
		env.pools, err = env.tsuru.PoolList()
		if err != nil {
			if env.config.verbose {
				fmt.Printf("Error fetching pools: %s\n", err)
			}
			return
		}
	}

	var nodeOps int
	for addr, evs := range group["node"] {
		endTime := evs[len(evs)-1].EndTime
		lastStatus := eventStatus(evs[len(evs)-1])
		op := &nodeOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			nodeAddr: addr,
		}
		operations = append(operations, op)
		nodeOps++
	}

	for addr, evs := range group["healer"] {
		endTime := evs[len(evs)-1].EndTime
		removedNodeOp := &nodeOperation{
			baseOperation: baseOperation{
				action: "DELETE",
				time:   endTime,
			},
			nodeAddr: addr,
		}
		operations = append(operations, removedNodeOp)

		var data map[string]string
		err := evs[0].EndData(&data)
		if err != nil {
			continue
		}
		addedNodeOp := &nodeOperation{
			baseOperation: baseOperation{
				action: "UPDATE",
				time:   endTime,
			},
			nodeAddr: data["_id"],
		}
		operations = append(operations, addedNodeOp)
		nodeOps++
	}

	if nodeOps > 0 {
		var err error
		env.nodes, err = env.tsuru.NodeList()
		if err != nil {
			if env.config.verbose {
				fmt.Printf("Error fetching nodes: %s\n", err)
			}
			return
		}
	}

	for name, evs := range group["app"] {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		endTime := evs[len(evs)-1].EndTime
		lastStatus := eventStatus(evs[len(evs)-1])

		var cachedApp *app
		if lastStatus != "DELETE" {
			var err error
			cachedApp, err = env.tsuru.AppInfo(name)
			if err != nil {
				continue
			}
		}

		op := &appOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			appName:   name,
			cachedApp: cachedApp,
		}
		operations = append(operations, op)

		appPoolOp := &appPoolOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			appName:   name,
			cachedApp: cachedApp,
		}
		operations = append(operations, appPoolOp)
	}

	postUpdates(operations)
}

func groupByTarget(events []event) map[string]groupedEvents {
	results := make(map[string]groupedEvents)

	for _, ev := range events {
		if ev.Failed() {
			continue
		}

		name := ev.Target.Value
		evType := ev.Target.Type

		if results[evType] == nil {
			results[evType] = groupedEvents{}
		}
		results[evType][name] = append(results[evType][name], ev)
	}

	return results
}
