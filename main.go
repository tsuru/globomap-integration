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

var (
	config configParams
	tsuru  *tsuruClient
	pools  []pool
)

func setup(args []string) {
	config = configParams{
		tsuruHostname:    os.Getenv("TSURU_HOSTNAME"),
		tsuruToken:       os.Getenv("TSURU_TOKEN"),
		globomapHostname: os.Getenv("GLOBOMAP_HOSTNAME"),
		startTime:        time.Now().Add(-24 * time.Hour),
	}
	err := config.processArguments(args)
	if err != nil {
		panic(err)
	}
	tsuru = &tsuruClient{
		Hostname: config.tsuruHostname,
		Token:    config.tsuruToken,
	}
}

func main() {
	setup(os.Args[1:])
	var cmd command
	cmd = &update{}
	cmd.Run()
}

func postUpdates(operations []operation) {
	globomap := globomapClient{
		Hostname: config.globomapHostname,
	}
	globomap.Post(operations)
}
