// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
)

type updateCmd struct{}

type groupedEvents map[string][]event

func (u *updateCmd) Run() {
	since := time.Now().Add(-1 * *env.config.start)

	if env.config.verbose {
		fmt.Printf("Fetching events since %s\n", since)
	}

	events := fetchEvents([]eventFilter{
		{Kindnames: []string{
			"app.create", "app.update", "app.delete",
			"pool.create", "pool.update", "pool.delete",
			"node.create", "node.delete",
			"service.create", "service.delete",
			"service-instance.create", "service-instance.delete",
		}, Since: &since},
		{Kindnames: []string{"healer"}, TargetType: "node", Since: &since},
	})

	if env.config.verbose {
		fmt.Printf("Found %d events\n", len(events))
	}

	processEvents(events, map[string]eventProcessorFunc{
		"pool":             processPoolEvents,
		"node":             processNodeEvents,
		"app":              processAppEvents,
		"service":          processorAsFunc(&serviceProcessor{}),
		"service-instance": processorAsFunc(&serviceInstanceProcessor{}),
	})
}

func fetchEvents(filters []eventFilter) []event {

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

type eventProcessor interface {
	process(groupedEvents) ([]operation, error)
}

func processorAsFunc(p eventProcessor) eventProcessorFunc {
	return p.process
}

type eventProcessorFunc func(groupedEvents) ([]operation, error)

// processEvents groups events by target and pass each of the groups to the
// corresponding processor
func processEvents(events []event, processors map[string]eventProcessorFunc) {

	group := groupByTarget(events)
	operations := []operation{}
	for g, p := range processors {
		ops, err := p(group[g])
		if err != nil {
			if env.config.verbose {
				fmt.Printf("Error processing %s events: %v", g, err)
			}
			continue
		}
		operations = append(operations, ops...)
	}

	postUpdates(operations)
}

type serviceInstanceProcessor struct {
	instances map[string]tsuru.ServiceInstance
}

func (p *serviceInstanceProcessor) process(events groupedEvents) ([]operation, error) {
	var operations []operation

	if len(events) > 0 && p.instances == nil {
		services, err := env.tsuru.ServiceList()
		if err != nil {
			return nil, err
		}

		p.instances = make(map[string]tsuru.ServiceInstance)

		for _, s := range services {
			for _, i := range s.ServiceInstances {
				p.instances[s.Service+"/"+i.Name] = i
			}
		}
	}

	for name, evs := range events {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		endTime := evs[len(evs)-1].EndTime
		lastStatus := eventStatus(evs[len(evs)-1])

		instance := p.instances[name]

		// we need to make sure we set the name even if the service
		// instance was deleted (and is not in the map)
		nameParts := strings.Split(name, "/")
		if len(nameParts) != 2 {
			return nil, fmt.Errorf("invalid instance name %q from event", name)
		}
		instance.ServiceName = nameParts[0]
		instance.Name = nameParts[1]

		op := serviceInstanceOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			instance: instance,
		}

		operations = append(operations, &op)
	}

	return operations, nil
}

type serviceProcessor struct {
	services map[string]tsuru.Service
}

func (p *serviceProcessor) process(events groupedEvents) ([]operation, error) {
	var operations []operation

	if len(events) > 0 && p.services == nil {
		services, err := env.tsuru.ServiceList()
		if err != nil {
			return nil, err
		}
		p.services = make(map[string]tsuru.Service)
		for _, s := range services {
			p.services[s.Service] = s
		}
	}

	for name, evs := range events {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		endTime := evs[len(evs)-1].EndTime
		lastStatus := eventStatus(evs[len(evs)-1])
		service := p.services[name]

		// we need to make sure we set the name even if the service
		// was deleted (and is not in the map)
		service.Service = name

		op := serviceOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			service: service,
		}

		operations = append(operations, &op)
	}

	return operations, nil
}

func processPoolEvents(events groupedEvents) ([]operation, error) {
	var operations []operation

	for name, evs := range events {
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
			return nil, err
		}
	}

	return operations, nil
}

func processNodeEvents(events groupedEvents) ([]operation, error) {
	var operations []operation
	for addr, evs := range events {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].EndTime.Unix() < evs[j].EndTime.Unix()
		})
		lastEvent := evs[0]
		endTime := lastEvent.EndTime

		if lastEvent.Kind.Name == "healer" {
			if ops, err := processHealerEvent(lastEvent, addr); err == nil {
				operations = append(operations, ops...)
			} else {
				fmt.Printf("Error processing healing event for addr %v: %v", addr, err)
			}
			continue
		}

		lastStatus := eventStatus(lastEvent)
		op := &nodeOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			nodeAddr: addr,
		}
		operations = append(operations, op)
	}

	if len(operations) > 0 {
		var err error
		env.nodes, err = env.tsuru.NodeList()
		if err != nil {
			return nil, err
		}
	}

	return operations, nil
}

func processHealerEvent(e event, addr string) ([]operation, error) {
	endTime := e.EndTime

	removedNodeOp := &nodeOperation{
		baseOperation: baseOperation{
			action: "DELETE",
			time:   endTime,
		},
		nodeAddr: addr,
	}

	var data map[string]string
	err := e.EndData(&data)
	if err != nil {
		return nil, err
	}
	addedNodeOp := &nodeOperation{
		baseOperation: baseOperation{
			action: "UPDATE",
			time:   endTime,
		},
		nodeAddr: data["_id"],
	}

	return append([]operation{}, addedNodeOp, removedNodeOp), nil
}

func processAppEvents(events groupedEvents) ([]operation, error) {
	var operations []operation
	for name, evs := range events {
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
				if env.config.verbose {
					fmt.Printf("Failed to retrieve app %s info: %v. Skipping.", name, err)
				}
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
		appPoolOp := &appPoolOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			appName:   name,
			cachedApp: cachedApp,
		}
		operations = append(operations, op, appPoolOp)
	}

	return operations, nil
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
