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
	events := make(chan []event, len(kindnames))
	for _, kindname := range kindnames {
		go func(kindname string) {
			f := eventFilter{
				Kindname: kindname,
				Since:    &startTime,
			}
			ev, err := tsuru.EventList(f)
			if err != nil {
				events <- nil
				return
			}
			events <- ev
		}(kindname)
	}

	eventList := []event{}
	for i := 0; i < len(kindnames); i++ {
		eventList = append(eventList, <-events...)
	}

	for _, event := range eventList {
		fmt.Printf("%s\t%s\t%s\n", event.StartTime, event.Kind.Name, event.Target.Value)
	}
}
