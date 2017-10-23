// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"gopkg.in/check.v1"
)

func sortPayload(data []globomapPayload) {
	sort.Slice(data, func(i, j int) bool {
		collection1, _ := data[i]["collection"].(string)
		collection2, _ := data[j]["collection"].(string)
		if collection1 != collection2 {
			return collection1 < collection2
		}
		el, _ := data[i]["element"].(map[string]interface{})
		id1, _ := el["id"].(string)
		el, _ = data[j]["element"].(map[string]interface{})
		id2, _ := el["id"].(string)
		return id1 < id2
	})
}

func newEvent(kind, value string) event {
	parts := strings.Split(kind, ".")
	e := event{}
	e.Target.Type = parts[0]
	e.Target.Value = value
	e.Kind.Name = kind
	return e
}

func (s *S) TestProcessEvents(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r, err := regexp.Compile("/apps/(.*)")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m := r.FindStringSubmatch(req.URL.Path)
		if len(m) < 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		name := m[1]
		a1 := app{Name: "myapp1", Pool: "pool1"}
		a2 := app{Name: "myapp2", Pool: "pool1"}
		switch name {
		case "myapp1":
			json.NewEncoder(w).Encode(a1)
		case "myapp2":
			json.NewEncoder(w).Encode(a2)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)
	setup(nil)

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 5)

		sortPayload(data)

		el, ok := data[0]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[0]["action"], check.Equals, "CREATE")
		c.Assert(data[0]["collection"], check.Equals, "tsuru_app")
		c.Assert(data[0]["type"], check.Equals, "collections")
		c.Assert(el["name"], check.Equals, "myapp1")

		el, ok = data[1]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[1]["action"], check.Equals, "DELETE")
		c.Assert(data[1]["collection"], check.Equals, "tsuru_app")
		c.Assert(data[1]["type"], check.Equals, "collections")
		c.Assert(el["name"], check.Equals, "myapp2")

		el, ok = data[2]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[2]["action"], check.Equals, "UPDATE")
		c.Assert(data[2]["collection"], check.Equals, "tsuru_pool")
		c.Assert(data[2]["type"], check.Equals, "collections")
		c.Assert(el["name"], check.Equals, "pool1")

		el, ok = data[3]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[3]["action"], check.Equals, "CREATE")
		c.Assert(data[3]["collection"], check.Equals, "tsuru_pool_app")
		c.Assert(data[3]["type"], check.Equals, "edges")
		c.Assert(el["name"], check.Equals, "myapp1-pool1")
		c.Assert(el["from"], check.Equals, "myapp1")
		c.Assert(el["to"], check.Equals, "pool1")

		el, ok = data[4]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[4]["action"], check.Equals, "DELETE")
		c.Assert(data[4]["collection"], check.Equals, "tsuru_pool_app")
		c.Assert(data[4]["type"], check.Equals, "edges")
		c.Assert(el["name"], check.Equals, "myapp2-pool1")
		c.Assert(el["from"], check.Equals, "myapp2")
		c.Assert(el["to"], check.Equals, "pool1")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_HOSTNAME", server.URL)

	processEvents([]event{
		newEvent("app.create", "myapp1"),
		newEvent("app.delete", "myapp2"),
		newEvent("pool.update", "pool1"),
	})
	c.Assert(atomic.LoadInt32(&requests), check.Equals, int32(1))
}

func (s *S) TestProcessEventsWithMultipleEventsPerKind(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r, err := regexp.Compile("/apps/(.*)")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m := r.FindStringSubmatch(req.URL.Path)
		if len(m) < 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		name := m[1]
		a1 := app{Name: "myapp1", Pool: "pool1"}
		a2 := app{Name: "myapp2", Pool: "pool1"}
		switch name {
		case "myapp1":
			json.NewEncoder(w).Encode(a1)
		case "myapp2":
			json.NewEncoder(w).Encode(a2)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)
	setup(nil)

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 5)

		sortPayload(data)

		el, ok := data[0]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[0]["action"], check.Equals, "DELETE")
		c.Assert(data[0]["collection"], check.Equals, "tsuru_app")
		c.Assert(data[0]["type"], check.Equals, "collections")
		c.Assert(el["name"], check.Equals, "myapp1")

		el, ok = data[1]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[1]["action"], check.Equals, "CREATE")
		c.Assert(data[1]["collection"], check.Equals, "tsuru_app")
		c.Assert(data[1]["type"], check.Equals, "collections")
		c.Assert(el["name"], check.Equals, "myapp2")

		el, ok = data[2]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[2]["action"], check.Equals, "UPDATE")
		c.Assert(data[2]["collection"], check.Equals, "tsuru_pool")
		c.Assert(data[2]["type"], check.Equals, "collections")
		c.Assert(el["name"], check.Equals, "pool1")

		el, ok = data[3]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[3]["action"], check.Equals, "DELETE")
		c.Assert(data[3]["collection"], check.Equals, "tsuru_pool_app")
		c.Assert(data[3]["type"], check.Equals, "edges")
		c.Assert(el["name"], check.Equals, "myapp1-pool1")
		c.Assert(el["from"], check.Equals, "myapp1")
		c.Assert(el["to"], check.Equals, "pool1")

		el, ok = data[4]["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data[4]["action"], check.Equals, "CREATE")
		c.Assert(data[4]["collection"], check.Equals, "tsuru_pool_app")
		c.Assert(data[4]["type"], check.Equals, "edges")
		c.Assert(el["name"], check.Equals, "myapp2-pool1")
		c.Assert(el["from"], check.Equals, "myapp2")
		c.Assert(el["to"], check.Equals, "pool1")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_HOSTNAME", server.URL)

	processEvents([]event{
		newEvent("app.update", "myapp1"),
		newEvent("app.delete", "myapp1"),
		newEvent("app.create", "myapp2"),
		newEvent("app.update", "myapp2"),
		newEvent("pool.update", "pool1"),
		newEvent("pool.delete", "pool1"),
		newEvent("pool.create", "pool1"),
		newEvent("pool.create", "pool2"),
		newEvent("pool.delete", "pool2"),
	})
	c.Assert(atomic.LoadInt32(&requests), check.Equals, int32(1))
}
