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
	kindnames := []string{"app.create", "app.update", "app.delete"}
	f := eventFilter{
		Since: &startTime,
	}
	var events []event
	for _, kindname := range kindnames {
		f.Kindname = kindname
		ev, err := tsuru.EventList(f)
		fmt.Printf("%s %d\n", kindname, len(ev))
		if err != nil {
			fmt.Println(err)
			return
		}
		events = append(events, ev...)
	}

	for _, event := range events {
		fmt.Printf("%s\t%s\t%s\n", event.StartTime, event.Kind.Name, event.Target.Value)
	}
}
