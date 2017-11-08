// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"gopkg.in/check.v1"
)

func (s *S) TestEventList(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodGet)
		c.Assert(r.URL.Path, check.Equals, "/events")
		c.Assert(r.FormValue("running"), check.Equals, "false")
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
		c.Assert(r.ParseForm(), check.IsNil)
		c.Assert(r.FormValue("running"), check.Equals, "false")
		c.Assert(r.Form["kindname"], check.DeepEquals, []string{"app.update", "app.create"})
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
		Kindnames: []string{"app.update", "app.create"},
		Since:     &since,
		Until:     &until,
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

func (s *S) TestAppList(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodGet)
		c.Assert(r.URL.Path, check.Equals, "/apps")
		c.Assert(r.Header.Get("Authorization"), check.Equals, "b "+s.token)

		a1 := app{Name: "app1"}
		a2 := app{Name: "app2"}
		json.NewEncoder(w).Encode([]app{a1, a2})
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	apps, err := client.AppList()
	c.Assert(err, check.IsNil)
	c.Assert(apps, check.HasLen, 2)
	c.Assert(apps[0].Name, check.Equals, "app1")
	c.Assert(apps[1].Name, check.Equals, "app2")
}

func (s *S) TestAppListError(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	apps, err := client.AppList()
	c.Assert(err, check.NotNil)
	c.Assert(apps, check.HasLen, 0)
}

func (s *S) TestAppInfo(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodGet)
		c.Assert(r.URL.Path, check.Equals, "/apps/test-app")
		c.Assert(r.Header.Get("Authorization"), check.Equals, "b "+s.token)

		a := app{Name: "test-app"}
		json.NewEncoder(w).Encode(a)
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	app, err := client.AppInfo("test-app")
	c.Assert(err, check.IsNil)
	c.Assert(app, check.NotNil)
	c.Assert(app.Name, check.Equals, "test-app")
}

func (s *S) TestAppInfoNotFound(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodGet)
		c.Assert(r.URL.Path, check.Equals, "/apps/test-app")
		c.Assert(r.Header.Get("Authorization"), check.Equals, "b "+s.token)

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	app, err := client.AppInfo("test-app")
	c.Assert(err, check.NotNil)
	c.Assert(app, check.IsNil)
}

func (s *S) TestPoolList(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodGet)
		c.Assert(r.URL.Path, check.Equals, "/pools")
		c.Assert(r.Header.Get("Authorization"), check.Equals, "b "+s.token)

		p1 := pool{Name: "pool1"}
		p2 := pool{Name: "pool2"}
		json.NewEncoder(w).Encode([]pool{p1, p2})
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	pools, err := client.PoolList()
	c.Assert(err, check.IsNil)
	c.Assert(pools, check.HasLen, 2)
	c.Assert(pools[0].Name, check.Equals, "pool1")
	c.Assert(pools[1].Name, check.Equals, "pool2")
}

func (s *S) TestPoolListError(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	pools, err := client.PoolList()
	c.Assert(err, check.NotNil)
	c.Assert(pools, check.HasLen, 0)
}

func (s *S) TestNodeList(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodGet)
		c.Assert(r.URL.Path, check.Equals, "/node")
		c.Assert(r.Header.Get("Authorization"), check.Equals, "b "+s.token)

		n1 := node{Id: "1234"}
		n2 := node{Id: "5678"}
		json.NewEncoder(w).Encode(struct{ Nodes []node }{[]node{n1, n2}})
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	nodes, err := client.NodeList()
	c.Assert(err, check.IsNil)
	c.Assert(nodes, check.HasLen, 2)
	c.Assert(nodes[0].Id, check.Equals, "1234")
	c.Assert(nodes[1].Id, check.Equals, "5678")
}

func (s *S) TestNodeListError(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	client := tsuruClient{
		Hostname: server.URL,
		Token:    s.token,
	}

	nodes, err := client.NodeList()
	c.Assert(err, check.NotNil)
	c.Assert(nodes, check.HasLen, 0)
}

func (s *S) TestEventFailed(c *check.C) {
	successfulEvent := event{}
	failedEvent := event{Error: "some error"}
	c.Assert(successfulEvent.Failed(), check.Equals, false)
	c.Assert(failedEvent.Failed(), check.Equals, true)
}

func (s *S) TestEventPoolName(c *check.C) {
	e := event{}
	c.Assert(e.PoolName(), check.Equals, "")

	e.Allowed.Scheme = "pool.read.events"
	e.Allowed.Contexts = append(e.Allowed.Contexts, struct{ CtxType, Value string }{"app", "myapp"})
	c.Assert(e.PoolName(), check.Equals, "")

	e.Allowed.Scheme = "something"
	e.Allowed.Contexts = append(e.Allowed.Contexts, struct{ CtxType, Value string }{"pool", "mypool"})
	c.Assert(e.PoolName(), check.Equals, "")

	e.Allowed.Scheme = "pool.read.events"
	c.Assert(e.PoolName(), check.Equals, "mypool")
}

func (s *S) TestAppAddresses(c *check.C) {
	a := app{Ip: "ip", Cname: []string{"addr1", "addr2"}}
	c.Assert(a.Addresses(), check.DeepEquals, []string{"addr1", "addr2", "ip"})
}

func (s *S) TestNodeName(c *check.C) {
	n := node{Metadata: nodeMetadata{IaasID: "vm-1234"}}
	c.Assert(n.Name(), check.Equals, "vm-1234")
}

func (s *S) TestNodeAddr(c *check.C) {
	n1 := node{Address: "10.2.1.153"}
	c.Assert(n1.Addr(), check.Equals, "10.2.1.153")

	n2 := node{Protocol: "http", Address: "10.2.1.30", Port: 0}
	c.Assert(n2.Addr(), check.Equals, "http://10.2.1.30")

	n3 := node{Protocol: "https", Address: "10.2.1.112", Port: 2376}
	c.Assert(n3.Addr(), check.Equals, "https://10.2.1.112:2376")

	n4 := node{Protocol: "", Address: "10.2.1.115", Port: 2376}
	c.Assert(n4.Addr(), check.Equals, "10.2.1.115:2376")
}
