// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"time"
)

type command interface {
	Run()
}

type environment struct {
	config configParams
	cmd    command
	tsuru  *tsuruClient
	pools  []pool
}

var env environment

func setup(args []string) {
	env = environment{
		cmd: &updateCmd{},
		config: configParams{
			tsuruHostname:    os.Getenv("TSURU_HOSTNAME"),
			tsuruToken:       os.Getenv("TSURU_TOKEN"),
			globomapHostname: os.Getenv("GLOBOMAP_HOSTNAME"),
			startTime:        time.Now().Add(-24 * time.Hour),
		},
	}
	err := env.config.processArguments(args)
	if err != nil {
		panic(err)
	}
	env.tsuru = &tsuruClient{
		Hostname: env.config.tsuruHostname,
		Token:    env.config.tsuruToken,
	}
}

func main() {
	setup(os.Args[1:])
	env.cmd.Run()
}

func postUpdates(operations []operation) {
	globomap := globomapClient{
		Hostname: env.config.globomapHostname,
	}
	data := []globomapPayload{}
	for _, op := range operations {
		payload := op.toPayload()
		if len(payload) > 0 {
			data = append(data, payload...)
		}
	}
	globomap.Post(data)
}
