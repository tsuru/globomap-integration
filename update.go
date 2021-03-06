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

	events = fetchEvents([]eventFilter{
		{Kindnames: []string{"app.update.bind", "app.update.unbind"}, Since: &since},
	})

	if env.config.verbose {
		fmt.Printf("Found %d bind/unbind events\n", len(events))
	}

	processEvents(events, map[string]eventProcessorFunc{
		"app": processAppInstanceEvents,
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
	process(target string, events []event) ([]operation, error)
}

func processorAsFunc(p eventProcessor) eventProcessorFunc {
	return p.process
}

type eventProcessorFunc func(target string, events []event) ([]operation, error)

// processEvents groups events by target and pass each of the groups to the
// corresponding processor
func processEvents(events []event, processors map[string]eventProcessorFunc) {

	group := groupByTarget(events)
	operations := []operation{}
	for g, p := range processors {
		for target, evs := range group[g] {
			sort.Slice(evs, func(i, j int) bool {
				return evs[i].EndTime.UnixNano() < evs[j].EndTime.UnixNano()
			})
			ops, err := p(target, evs)
			if err != nil {
				if env.config.verbose {
					fmt.Printf("[%v] Error processing %s events: %v", g, target, err)
				}
				continue
			}
			operations = append(operations, ops...)
		}
	}

	postUpdates(operations)
}

type serviceInstanceProcessor struct {
	instances map[string]tsuru.ServiceInstance
}

func (p *serviceInstanceProcessor) process(target string, events []event) ([]operation, error) {
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
	lastEvent := events[len(events)-1]
	endTime := lastEvent.EndTime
	lastStatus := eventStatus(lastEvent)

	instance := p.instances[target]

	// we need to make sure we set the name even if the service
	// instance was deleted (and is not in the map)
	service, instanceName, err := extractServiceInstance(target)
	if err != nil {
		return nil, fmt.Errorf("%v. Skipping event kind=%v target=%v", err, lastEvent.Kind.Name, lastEvent.Target.Value)
	}
	instance.ServiceName, instance.Name = service, instanceName

	op := serviceInstanceOperation{
		baseOperation: baseOperation{
			action: lastStatus,
			time:   endTime,
		},
		instance: instance,
	}

	op2 := serviceServiceInstanceOperation{
		baseOperation: baseOperation{
			action: lastStatus,
			time:   endTime,
		},
		instance: instance,
	}

	operations = append(operations, &op, &op2)

	return operations, nil
}

type serviceProcessor struct {
	services map[string]tsuru.Service
}

func (p *serviceProcessor) process(target string, events []event) ([]operation, error) {
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

	endTime := events[len(events)-1].EndTime
	lastStatus := eventStatus(events[len(events)-1])
	service := p.services[target]

	// we need to make sure we set the name even if the service
	// was deleted (and is not in the map)
	service.Service = target

	op := serviceOperation{
		baseOperation: baseOperation{
			action: lastStatus,
			time:   endTime,
		},
		service: service,
	}

	operations = append(operations, &op)

	return operations, nil
}

func processPoolEvents(target string, events []event) ([]operation, error) {
	var operations []operation

	endTime := events[len(events)-1].EndTime
	lastStatus := eventStatus(events[len(events)-1])
	op := &poolOperation{
		baseOperation: baseOperation{
			action: lastStatus,
			time:   endTime,
		},
		poolName: target,
	}
	operations = append(operations, op)

	if len(operations) > 0 {
		var err error
		env.pools, err = env.tsuru.PoolList()
		if err != nil {
			return nil, err
		}
	}

	return operations, nil
}

func processNodeEvents(target string, events []event) ([]operation, error) {
	var operations []operation

	lastEvent := events[len(events)-1]
	endTime := lastEvent.EndTime

	if lastEvent.Kind.Name == "healer" {
		if ops, err := processHealerEvent(lastEvent, target); err == nil {
			operations = append(operations, ops...)
		} else {
			fmt.Printf("Error processing healing event for addr %v: %v", target, err)
		}
		return operations, nil
	}

	lastStatus := eventStatus(lastEvent)
	op := &nodeOperation{
		baseOperation: baseOperation{
			action: lastStatus,
			time:   endTime,
		},
		nodeAddr: target,
	}
	operations = append(operations, op)

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

func processAppEvents(target string, events []event) ([]operation, error) {
	endTime := events[len(events)-1].EndTime
	lastStatus := eventStatus(events[len(events)-1])

	var cachedApp *app
	if lastStatus != "DELETE" {
		var err error
		cachedApp, err = env.tsuru.AppInfo(target)
		if err != nil {
			if env.config.verbose {
				fmt.Printf("Failed to retrieve app %s info: %v. Skipping.", target, err)
			}
			return nil, nil
		}
	}

	operations := []operation{
		&appOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			appName:   target,
			cachedApp: cachedApp,
		},
		&appPoolOperation{
			baseOperation: baseOperation{
				action: lastStatus,
				time:   endTime,
			},
			appName:   target,
			cachedApp: cachedApp,
		},
	}

	return operations, nil
}

func extractServiceInstance(fqdn string) (string, string, error) {
	parts := strings.SplitN(fqdn, "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("failed to extract service instance from %q", fqdn)
	}
	return parts[0], parts[1], nil
}

func processAppInstanceEvents(app string, events []event) ([]operation, error) {
	// We only care about the last bind/unbind operation to this
	// service instance by this app.
	operations := make(map[string]operation)
	for _, e := range events {
		action := "UPDATE"
		if strings.HasSuffix(e.Kind.Name, "unbind") {
			action = "DELETE"
		}
		data := []map[string]interface{}{}
		var service, instance string
		if err := e.StartCustomData.Unmarshal(&data); err != nil {
			return nil, err
		}
		for _, d := range data {
			if d["name"] == ":service" {
				service = d["value"].(string)
			}
			if d["name"] == ":instance" {
				instance = d["value"].(string)
			}
			if service != "" && instance != "" {
				break
			}
		}
		if service == "" || instance == "" {
			return nil, fmt.Errorf("Unable to extract service and instance from data: %v", data)
		}
		operations[e.Target.Value] = &appServiceInstanceOperation{
			baseOperation: baseOperation{
				action: action,
				time:   e.EndTime,
			},
			appName:      app,
			serviceName:  service,
			instanceName: instance,
		}
	}

	var opList []operation
	for _, o := range operations {
		opList = append(opList, o)
	}
	return opList, nil
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
