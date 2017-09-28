// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gopkg.in/check.v1"
)

type S struct {
	token string
}

var _ = check.Suite(&S{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *S) SetUpSuite(c *check.C) {
	s.token = "mytoken"
}

func (s *S) TestEventList(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodGet)
		c.Assert(r.URL.Path, check.Equals, "/events")
		c.Assert(r.Header.Get("Authorization"), check.Equals, "b "+s.token)

		e1 := event{}
		e1.Target.Value = "myapp1"
		e2 := event{}
		e2.Target.Value = "myapp2"
		json.NewEncoder(w).Encode([]event{e1, e2})
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	events, err := client.EventList(eventFilter{})
	c.Assert(err, check.IsNil)
	c.Assert(events, check.HasLen, 2)
	c.Assert(events[0].Target.Value, check.Equals, "myapp1")
	c.Assert(events[1].Target.Value, check.Equals, "myapp2")
}

func (s *S) TestEventListWithFilters(c *check.C) {
	until := time.Now()
	since := until.Add(-1 * time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.FormValue("kindname"), check.Equals, "app.update")
		c.Assert(r.FormValue("since"), check.Equals, since.Format(TIME_FORMAT))
		c.Assert(r.FormValue("until"), check.Equals, until.Format(TIME_FORMAT))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	filter := eventFilter{
		Kindname: "app.update",
		Since:    &since,
		Until:    &until,
	}
	_, err := client.EventList(filter)
	c.Assert(err, check.IsNil)
}

func (s *S) TestEventListNoContent(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	events, err := client.EventList(eventFilter{})
	c.Assert(err, check.IsNil)
	c.Assert(events, check.HasLen, 0)
}
