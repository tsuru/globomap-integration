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
		op := NewTsuruOperation(nil)
		op.target = &appOperation{appName: app.Name}
		operations[i] = op
	}
	i := len(apps)
	for k, pool := range env.pools {
		op := NewTsuruOperation(nil)
		op.target = &poolOperation{poolName: pool.Name}
		operations[i+k] = op
	}
	postUpdates(operations)
}
