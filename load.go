// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

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

	operations := make([]operation, len(apps)+len(env.pools))
	for i, app := range apps {
		operations[i] = NewOperation(nil)
		operations[i].target = &appOperation{appName: app.Name}
	}
	i := len(apps)
	for k, pool := range env.pools {
		operations[i+k] = NewOperation(nil)
		operations[i+k].target = &poolOperation{poolName: pool.Name}
	}
	postUpdates(operations)
}
