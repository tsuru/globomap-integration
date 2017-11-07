// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"time"
)

type command interface {
	Run()
}

type environment struct {
	config   configParams
	cmd      command
	tsuru    *tsuruClient
	globomap *globomapClient
	pools    []pool
	nodes    []node
}

var env environment

func setup(args []string) {
	env = environment{
		config: NewConfig(),
	}
	err := env.config.ProcessArguments(args)
	if err != nil {
		panic(err)
	}
	env.tsuru = &tsuruClient{
		Hostname: env.config.tsuruHostname,
		Token:    env.config.tsuruToken,
	}
	env.globomap = &globomapClient{
		ApiHostname:    env.config.globomapApiHostname,
		LoaderHostname: env.config.globomapLoaderHostname,
	}
}

func main() {
	setup(os.Args[1:])

	if env.config.repeat == nil {
		env.cmd.Run()
	} else {
		for {
			start := time.Now()
			env.cmd.Run()
			diff := *env.config.repeat - time.Since(start)
			if diff > 0 {
				fmt.Printf("waiting %s...\n", diff)
				time.Sleep(diff)
			}
		}
	}
}

func postUpdates(operations []operation) {
	data := []globomapPayload{}
	for _, op := range operations {
		payload := op.toPayload()
		if len(payload) > 0 {
			data = append(data, payload...)
		}
	}
	env.globomap.Post(data)
}
