// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	tsuru := &tsuruClient{
		Hostname: os.Getenv("TSURU_HOSTNAME"),
		Token:    os.Getenv("TSURU_TOKEN"),
	}
	startTime := time.Now().Add(-24 * time.Hour)
	f := eventFilter{
		Kindname: "app.create",
		Since:    &startTime,
	}
	events, err := tsuru.EventList(f)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, event := range events {
		fmt.Printf("%s %s\n", event.StartTime, event.Target.Value)
	}
}
