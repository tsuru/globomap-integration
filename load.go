// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"time"
)

type loadCmd struct{}

func (l *loadCmd) Run() {
	apps, err := env.tsuru.AppList()
	if err != nil {
		fmt.Printf("Error fetching apps: %s\n", err)
		return
	}
	env.pools, err = env.tsuru.PoolList()
	if err != nil {
		fmt.Printf("Error fetching pools: %s\n", err)
		return
	}
	env.nodes, err = env.tsuru.NodeList()
	if err != nil {
		fmt.Printf("Error fetching nodes: %s\n", err)
		return
	}

	operations := make([]operation, len(apps)+len(env.pools)+len(env.nodes))
	var i int
	for _, app := range apps {
		op := NewTsuruOperation(nil)
		op.target = &appOperation{appName: app.Name}
		operations[i] = op
		i++
	}
	for _, pool := range env.pools {
		op := NewTsuruOperation(nil)
		op.target = &poolOperation{poolName: pool.Name}
		operations[i] = op
		i++
	}
	for _, node := range env.nodes {
		op := &nodeOperation{
			action:   "UPDATE",
			time:     time.Now(),
			nodeAddr: node.Addr(),
		}
		operations[i] = op
		i++
	}
	postUpdates(operations)
}
