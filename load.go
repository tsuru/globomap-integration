// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sync"
	"time"
)

type loadCmd struct {
	wg sync.WaitGroup
}

func (c *loadCmd) Run() {
	c.wg.Add(4)

	go c.loadApps()
	go c.loadPools()
	go c.loadNodes()
	go c.loadServices()

	c.wg.Wait()
}

func (c *loadCmd) loadApps() {
	defer c.wg.Done()
	apps, err := env.tsuru.AppList()
	if err != nil {
		if env.config.verbose {
			fmt.Printf("Error fetching apps: %s\n", err)
		}
		return
	}

	if len(apps) == 0 {
		if env.config.verbose {
			fmt.Println("No apps to process")
		}
		return
	}
	if env.config.verbose {
		fmt.Printf("Processing %d apps\n", len(apps))
	}

	appOps := make([]operation, 2*len(apps))
	var i int
	for _, app := range apps {
		cachedApp, err := env.tsuru.AppInfo(app.Name)
		if err != nil {
			if env.config.verbose {
				fmt.Printf("Error fetching app %s info: %s\n", app.Name, err)
			}
			continue
		}

		op := &appOperation{
			baseOperation: baseOperation{
				action: "UPDATE",
				time:   time.Now(),
			},
			appName:   cachedApp.Name,
			cachedApp: cachedApp,
		}
		appOps[i] = op
		i++

		appPoolOp := &appPoolOperation{
			baseOperation: baseOperation{
				action: "UPDATE",
				time:   time.Now(),
			},
			appName:   cachedApp.Name,
			cachedApp: cachedApp,
		}
		appOps[i] = appPoolOp
		i++
	}
	postUpdates(appOps)
}

func (c *loadCmd) loadPools() {
	defer c.wg.Done()
	var err error
	env.pools, err = env.tsuru.PoolList()
	if err != nil {
		if env.config.verbose {
			fmt.Printf("Error fetching pools: %s\n", err)
		}
		return
	}

	if len(env.pools) == 0 {
		if env.config.verbose {
			fmt.Println("No pools to process")
		}
		return
	}
	if env.config.verbose {
		fmt.Printf("Processing %d pools\n", len(env.pools))
	}

	poolOps := make([]operation, len(env.pools))
	var i int
	for _, pool := range env.pools {
		op := &poolOperation{
			baseOperation: baseOperation{
				action: "UPDATE",
				time:   time.Now(),
			},
			poolName: pool.Name,
		}
		poolOps[i] = op
		i++
	}
	postUpdates(poolOps)
}

func (c *loadCmd) loadNodes() {
	defer c.wg.Done()
	var err error
	env.nodes, err = env.tsuru.NodeList()
	if err != nil {
		if env.config.verbose {
			fmt.Printf("Error fetching nodes: %s\n", err)
		}
		return
	}

	if len(env.nodes) == 0 {
		if env.config.verbose {
			fmt.Println("No nodes to process")
		}
		return
	}
	if env.config.verbose {
		fmt.Printf("Processing %d nodes\n", len(env.nodes))
	}

	nodeOps := make([]operation, len(env.nodes))
	var i int
	for _, node := range env.nodes {
		op := &nodeOperation{
			baseOperation: baseOperation{
				action: "UPDATE",
				time:   time.Now(),
			},
			nodeAddr: node.Addr(),
		}
		nodeOps[i] = op
		i++
	}
	postUpdates(nodeOps)
}

func (c *loadCmd) loadServices() {
	defer c.wg.Done()
	services, err := env.tsuru.ServiceList()
	if err != nil {
		if env.config.verbose {
			fmt.Printf("Error fetching services: %s\n", err)
		}
		return
	}

	if len(services) == 0 {
		if env.config.verbose {
			fmt.Println("No services to process")
		}
		return
	}

	if env.config.verbose {
		fmt.Printf("Processing %d services\n", len(services))
	}

	serviceOps := make([]operation, len(services))
	var instanceOps []operation
	var serviceInstanceOps []operation
	var appInstanceOps []operation
	for i := range services {
		serviceOps[i] = &serviceOperation{
			baseOperation: baseOperation{
				action: "UPDATE",
				time:   time.Now(),
			},
			service: services[i],
		}

		for _, instance := range services[i].ServiceInstances {
			instanceOps = append(instanceOps, &serviceInstanceOperation{
				baseOperation: baseOperation{
					action: "UPDATE",
					time:   time.Now(),
				},
				instance: instance,
			})

			serviceInstanceOps = append(serviceInstanceOps, &serviceServiceInstanceOperation{
				baseOperation: baseOperation{
					action: "UPDATE",
					time:   time.Now(),
				},
				instance: instance,
			})

			for _, app := range instance.Apps {
				appInstanceOps = append(appInstanceOps, &appServiceInstanceOperation{
					baseOperation: baseOperation{
						action: "UPDATE",
						time:   time.Now(),
					},
					appName:      app,
					instanceName: instance.Name,
					serviceName:  instance.ServiceName,
				})
			}

		}
	}
	postUpdates(instanceOps)
	postUpdates(serviceOps)
	postUpdates(serviceInstanceOps)
	postUpdates(appInstanceOps)
}
