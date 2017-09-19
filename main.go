// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
)

func main() {
	tsuru := &tsuruClient{
		Hostname: os.Getenv("TSURU_HOSTNAME"),
		Token:    os.Getenv("TSURU_TOKEN"),
	}
	events, err := tsuru.EventList()
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, event := range events {
		if event.Target.Type == "app" {
			fmt.Println(event.Target.Value)
		}
	}
}
