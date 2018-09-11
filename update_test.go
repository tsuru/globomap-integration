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
	"strings"
	"sync/atomic"
	"time"

	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
	"gopkg.in/check.v1"
	"gopkg.in/mgo.v2/bson"
)

func newEvent(kind, value string) event {
	parts := strings.Split(kind, ".")
	e := event{}
	e.Target.Type = parts[0]
	e.Target.Value = value
	e.Kind.Name = kind
	return e
}

func (s *S) TestUpdateCmdRun(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/events":
			if req.FormValue("target.type") == "" {
				events := []event{
					newEvent("app.create", "myapp1"),
					newEvent("app.delete", "myapp2"),
					newEvent("pool.update", "pool1"),
					newEvent("pool.delete", "pool2"),
				}
				json.NewEncoder(w).Encode(events)
			} else {
				json.NewEncoder(w).Encode(nil)
			}
		case "/1.0/apps/myapp1":
			json.NewEncoder(w).Encode(app{Name: "myapp1", Pool: "pool1"})
		case "/1.0/pools":
			json.NewEncoder(w).Encode([]pool{{Name: "pool1"}, {Name: "pool2"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 6)

		sortPayload(data)
		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_app")
		c.Assert(data[0].Type, check.Equals, "collections")
		c.Assert(data[0].Key, check.Equals, "tsuru_myapp1")
		c.Assert(el["name"], check.Equals, "myapp1")

		c.Assert(data[1].Action, check.Equals, "DELETE")
		c.Assert(data[1].Collection, check.Equals, "tsuru_app")
		c.Assert(data[1].Type, check.Equals, "collections")
		c.Assert(data[1].Key, check.Equals, "tsuru_myapp2")

		el = data[2].Element
		c.Assert(data[2].Action, check.Equals, "UPDATE")
		c.Assert(data[2].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[2].Type, check.Equals, "collections")
		c.Assert(data[2].Key, check.Equals, "tsuru_pool1")
		c.Assert(el["name"], check.Equals, "pool1")

		c.Assert(data[3].Action, check.Equals, "DELETE")
		c.Assert(data[3].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[3].Type, check.Equals, "collections")
		c.Assert(data[3].Key, check.Equals, "tsuru_pool2")

		el = data[4].Element
		c.Assert(data[4].Action, check.Equals, "UPDATE")
		c.Assert(data[4].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[4].Type, check.Equals, "edges")
		c.Assert(data[4].Key, check.Equals, "tsuru_myapp1-pool")
		c.Assert(el["name"], check.Equals, "myapp1-pool")
		c.Assert(el["from"], check.Equals, "tsuru_app/tsuru_myapp1")
		c.Assert(el["to"], check.Equals, "tsuru_pool/tsuru_pool1")

		c.Assert(data[5].Action, check.Equals, "DELETE")
		c.Assert(data[5].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[5].Type, check.Equals, "edges")
		c.Assert(data[5].Key, check.Equals, "tsuru_myapp2-pool")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup(nil)

	cmd := &updateCmd{}
	cmd.Run()

	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		c.Fail()
	}
}

func (s *S) TestUpdateCmdRunWithMultipleEventsPerKind(c *check.C) {
	var requestAppInfo1, requestAppInfo2 int32
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/events":
			if req.FormValue("target.type") == "" {
				events := []event{
					newEvent("app.update", "myapp1"),
					newEvent("app.delete", "myapp1"),
					newEvent("app.create", "myapp2"),
					newEvent("app.update", "myapp2"),
					newEvent("pool.update", "pool1"),
					newEvent("pool.delete", "pool1"),
					newEvent("pool.create", "pool1"),
					newEvent("pool.create", "pool2"),
					newEvent("pool.delete", "pool2"),
				}
				json.NewEncoder(w).Encode(events)
			} else {
				json.NewEncoder(w).Encode(nil)
			}
		case "/1.0/apps/myapp1":
			atomic.AddInt32(&requestAppInfo1, 1)
			json.NewEncoder(w).Encode(app{Name: "myapp1", Pool: "pool1"})
		case "/1.0/apps/myapp2":
			atomic.AddInt32(&requestAppInfo2, 1)
			json.NewEncoder(w).Encode(app{Name: "myapp2", Pool: "pool1"})
		case "/1.0/pools":
			json.NewEncoder(w).Encode([]pool{{Name: "pool1"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 6)

		sortPayload(data)
		c.Assert(data[0].Action, check.Equals, "DELETE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_app")
		c.Assert(data[0].Type, check.Equals, "collections")
		c.Assert(data[0].Key, check.Equals, "tsuru_myapp1")

		el := data[1].Element
		c.Assert(data[1].Action, check.Equals, "UPDATE")
		c.Assert(data[1].Collection, check.Equals, "tsuru_app")
		c.Assert(data[1].Type, check.Equals, "collections")
		c.Assert(data[1].Key, check.Equals, "tsuru_myapp2")
		c.Assert(el["name"], check.Equals, "myapp2")

		el = data[2].Element
		c.Assert(data[2].Action, check.Equals, "UPDATE")
		c.Assert(data[2].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[2].Type, check.Equals, "collections")
		c.Assert(data[2].Key, check.Equals, "tsuru_pool1")
		c.Assert(el["name"], check.Equals, "pool1")

		c.Assert(data[3].Action, check.Equals, "DELETE")
		c.Assert(data[3].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[3].Type, check.Equals, "collections")
		c.Assert(data[3].Key, check.Equals, "tsuru_pool2")

		c.Assert(data[4].Action, check.Equals, "DELETE")
		c.Assert(data[4].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[4].Type, check.Equals, "edges")
		c.Assert(data[4].Key, check.Equals, "tsuru_myapp1-pool")

		el = data[5].Element
		c.Assert(data[5].Action, check.Equals, "UPDATE")
		c.Assert(data[5].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[5].Type, check.Equals, "edges")
		c.Assert(data[5].Key, check.Equals, "tsuru_myapp2-pool")
		c.Assert(el["name"], check.Equals, "myapp2-pool")
		c.Assert(el["from"], check.Equals, "tsuru_app/tsuru_myapp2")
		c.Assert(el["to"], check.Equals, "tsuru_pool/tsuru_pool1")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup(nil)

	cmd := &updateCmd{}
	cmd.Run()

	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		c.Fail()
	}
	c.Assert(atomic.LoadInt32(&requestAppInfo1), check.Equals, int32(1))
	c.Assert(atomic.LoadInt32(&requestAppInfo2), check.Equals, int32(1))
}

func (s *S) TestUpdateCmdRunNoRequestWhenNoEventsToPost(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/events" {
			json.NewEncoder(w).Encode([]event{})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.ExpectFailure("No request should have been done")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup(nil)

	cmd := &updateCmd{}
	cmd.Run()

	select {
	case <-requests:
		c.Fail()
	case <-time.After(1 * time.Second):
	}
}

func (s *S) TestUpdateCmdRunAppProperties(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/events":
			if req.FormValue("target.type") == "" {
				events := []event{
					newEvent("app.create", "myapp1"),
				}
				json.NewEncoder(w).Encode(events)
			} else {
				json.NewEncoder(w).Encode(nil)
			}
		case "/1.0/apps/myapp1":
			a := app{
				Name:        "myapp1",
				Description: "about my app",
				Tags:        []string{"tag1", "tag2"},
				Platform:    "go",
				Ip:          "myapp1.example.com",
				Cname:       []string{"myapp1.alias.com"},
				Router:      "galeb",
				Owner:       "me@example.com",
				TeamOwner:   "my-team",
				Teams:       []string{"team1", "team2"},
				Plan:        &tsuru.Plan{Name: "large", Router: "galeb1", Memory: 1073741824, Swap: 0, Cpushare: 1024},
			}
			json.NewEncoder(w).Encode(a)
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 2)

		sortPayload(data)
		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_app")
		c.Assert(data[0].Type, check.Equals, "collections")
		c.Assert(data[0].Key, check.Equals, "tsuru_myapp1")
		c.Assert(el["name"], check.Equals, "myapp1")
		props, ok := el["properties"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(props["description"], check.Equals, "about my app")
		c.Assert(props["tags"], check.DeepEquals, []interface{}{"tag1", "tag2"})
		c.Assert(props["platform"], check.Equals, "go")
		c.Assert(props["addresses"], check.DeepEquals, []interface{}{"myapp1.alias.com", "myapp1.example.com"})
		c.Assert(props["router"], check.Equals, "galeb")
		c.Assert(props["owner"], check.Equals, "me@example.com")
		c.Assert(props["team_owner"], check.Equals, "my-team")
		c.Assert(props["teams"], check.DeepEquals, []interface{}{"team1", "team2"})
		c.Assert(props["plan_name"], check.Equals, "large")
		c.Assert(props["plan_router"], check.Equals, "galeb1")
		c.Assert(props["plan_memory"], check.Equals, "1073741824")
		c.Assert(props["plan_swap"], check.Equals, "0")
		c.Assert(props["plan_cpushare"], check.Equals, "1024")
		_, ok = el["properties_metadata"]
		c.Assert(ok, check.Equals, true)

		el = data[1].Element
		c.Assert(data[1].Action, check.Equals, "UPDATE")
		c.Assert(data[1].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[1].Type, check.Equals, "edges")
		c.Assert(data[1].Key, check.Equals, "tsuru_myapp1-pool")
		c.Assert(el["name"], check.Equals, "myapp1-pool")
		_, ok = el["properties"]
		c.Assert(ok, check.Equals, false)
		_, ok = el["properties_metadata"]
		c.Assert(ok, check.Equals, false)
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup(nil)

	cmd := &updateCmd{}
	cmd.Run()

	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		c.Fail()
	}
}

func (s *S) TestUpdateCmdRunPoolProperties(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/events":
			if req.FormValue("target.type") == "" {
				events := []event{
					newEvent("pool.create", "pool1"),
				}
				json.NewEncoder(w).Encode(events)
			} else {
				json.NewEncoder(w).Encode(nil)
			}
		case "/1.0/pools":
			p := pool{
				Name:        "pool1",
				Provisioner: "docker",
				Default_:    false,
				Public:      true,
				Teams:       []string{"team1", "team2", "team3"},
			}
			json.NewEncoder(w).Encode([]pool{p})
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 1)

		sortPayload(data)
		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[0].Type, check.Equals, "collections")
		c.Assert(data[0].Key, check.Equals, "tsuru_pool1")
		c.Assert(el["name"], check.Equals, "pool1")
		props, ok := el["properties"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(props["provisioner"], check.Equals, "docker")
		c.Assert(props["default"], check.Equals, "false")
		c.Assert(props["public"], check.Equals, "true")
		c.Assert(props["teams"], check.DeepEquals, []interface{}{"team1", "team2", "team3"})
		_, ok = el["properties_metadata"]
		c.Assert(ok, check.Equals, true)
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup(nil)

	cmd := &updateCmd{}
	cmd.Run()

	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		c.Fail()
	}
}

func (s *S) TestUpdateCmdRunWithNodeEvents(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/events":
			if req.FormValue("target.type") == "" {
				e1 := newEvent("node.create", "1.1.1.1")
				e2 := newEvent("node.delete", "https://2.2.2.2:2376")
				e3 := newEvent("node.create", "3.3.3.3")
				json.NewEncoder(w).Encode([]event{e1, e2, e3})
			} else {
				e4 := newEvent("healer", "https://4.4.4.4:2376")
				e4.Kind.Name = "node"
				data := struct {
					Id string `bson:"_id"`
				}{"https://5.5.5.5:2376"}
				b, err := bson.Marshal(data)
				c.Assert(err, check.IsNil)
				e4.EndCustomData = bson.Raw{Data: b, Kind: 3}

				json.NewEncoder(w).Encode([]event{e4})
			}
		case "/1.0/pools":
			p1 := pool{
				Name:        "pool1",
				Provisioner: "docker",
				Default_:    false,
				Public:      true,
				Teams:       []string{"team1", "team2", "team3"},
			}
			p2 := pool{
				Name:        "pool2",
				Provisioner: "swarm",
				Default_:    false,
				Public:      false,
				Teams:       []string{"team1"},
			}
			json.NewEncoder(w).Encode([]pool{p1, p2})
		case "/1.2/node":
			n1 := node{Pool: "pool1", Iaasid: "node1", Address: "https://1.1.1.1:2376"}
			n3 := node{Pool: "pool2", Iaasid: "node3", Address: "3.3.3.3"}
			n5 := node{Pool: "pool2", Iaasid: "node5", Address: "https://5.5.5.5:2376"}
			json.NewEncoder(w).Encode(struct{ Nodes []node }{Nodes: []node{n1, n3, n5}})
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	globomapApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		re := regexp.MustCompile(`"value":"([^"]*)"`)
		matches := re.FindAllStringSubmatch(req.FormValue("query"), -1)
		c.Assert(matches, check.HasLen, 1)
		c.Assert(matches[0], check.HasLen, 2)

		name := matches[0][1]
		queryResult := []globomapQueryResult{}
		switch name {
		case "node1":
			queryResult = append(queryResult, globomapQueryResult{Id: "comp_unit/globomap_node1", Name: "node1", Properties: globomapProperties{IPs: []string{"1.1.1.1"}}})
		case "node3":
			queryResult = append(queryResult, globomapQueryResult{Id: "comp_unit/globomap_node3", Name: "node3", Properties: globomapProperties{IPs: []string{"3.3.3.3"}}})
		case "node5":
			queryResult = append(queryResult, globomapQueryResult{Id: "comp_unit/globomap_node5", Name: "node5", Properties: globomapProperties{IPs: []string{"5.5.5.5"}}})
		}
		json.NewEncoder(w).Encode(
			struct{ Documents []globomapQueryResult }{
				Documents: queryResult,
			},
		)
	}))
	defer globomapApi.Close()
	os.Setenv("GLOBOMAP_API_HOSTNAME", globomapApi.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer close(requests)
		c.Assert(req.Method, check.Equals, http.MethodPost)
		c.Assert(req.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(req.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer req.Body.Close()
		c.Assert(data, check.HasLen, 5)

		sortPayload(data)
		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[0].Type, check.Equals, "edges")
		c.Assert(data[0].Key, check.Equals, "tsuru_1_1_1_1")
		c.Assert(el["id"], check.Equals, "1.1.1.1")
		c.Assert(el["name"], check.Equals, "node1")
		c.Assert(el["from"], check.Equals, "tsuru_pool/tsuru_pool1")
		c.Assert(el["to"], check.Equals, "comp_unit/globomap_node1")
		props, ok := el["properties"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(props["address"], check.Equals, "https://1.1.1.1:2376")

		c.Assert(data[1].Action, check.Equals, "DELETE")
		c.Assert(data[1].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[1].Type, check.Equals, "edges")
		c.Assert(data[1].Key, check.Equals, "tsuru_2_2_2_2")

		el = data[2].Element
		c.Assert(data[2].Action, check.Equals, "UPDATE")
		c.Assert(data[2].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[2].Type, check.Equals, "edges")
		c.Assert(data[2].Key, check.Equals, "tsuru_3_3_3_3")
		c.Assert(el["id"], check.Equals, "3.3.3.3")
		c.Assert(el["name"], check.Equals, "node3")
		c.Assert(el["from"], check.Equals, "tsuru_pool/tsuru_pool2")
		c.Assert(el["to"], check.Equals, "comp_unit/globomap_node3")
		props, ok = el["properties"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(props["address"], check.Equals, "3.3.3.3")

		c.Assert(data[3].Action, check.Equals, "DELETE")
		c.Assert(data[3].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[3].Type, check.Equals, "edges")
		c.Assert(data[3].Key, check.Equals, "tsuru_4_4_4_4")

		el = data[4].Element
		c.Assert(data[4].Action, check.Equals, "UPDATE")
		c.Assert(data[4].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[4].Type, check.Equals, "edges")
		c.Assert(data[4].Key, check.Equals, "tsuru_5_5_5_5")
		c.Assert(el["id"], check.Equals, "5.5.5.5")
		c.Assert(el["name"], check.Equals, "node5")
		c.Assert(el["from"], check.Equals, "tsuru_pool/tsuru_pool2")
		c.Assert(el["to"], check.Equals, "comp_unit/globomap_node5")
		props, ok = el["properties"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(props["address"], check.Equals, "https://5.5.5.5:2376")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup(nil)

	cmd := &updateCmd{}
	cmd.Run()

	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		c.Fail()
	}
}

func (s *S) TestUpdateCmdRunWithRetry(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/events":
			if req.FormValue("target.type") == "" {
				e1 := newEvent("node.create", "1.1.1.1")
				json.NewEncoder(w).Encode([]event{e1})
			} else {
				json.NewEncoder(w).Encode(nil)
			}
		case "/1.0/pools":
			p1 := pool{
				Name:        "pool1",
				Provisioner: "docker",
				Default_:    false,
				Public:      true,
				Teams:       []string{"team1", "team2", "team3"},
			}
			json.NewEncoder(w).Encode([]pool{p1})
		case "/1.2/node":
			n1 := node{Pool: "pool1", Iaasid: "node1", Address: "https://1.1.1.1:2376"}
			json.NewEncoder(w).Encode(struct{ Nodes []node }{Nodes: []node{n1}})
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	var globomapApiRequests int32
	globomapApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		re := regexp.MustCompile(`"value":"([^"]*)"`)
		matches := re.FindAllStringSubmatch(req.FormValue("query"), -1)
		c.Assert(matches, check.HasLen, 1)
		c.Assert(matches[0], check.HasLen, 2)

		queryResult := []globomapQueryResult{}
		if atomic.AddInt32(&globomapApiRequests, 1) > 1 {
			queryResult = append(queryResult, globomapQueryResult{Id: "comp_unit/globomap_node1", Name: "node1", Properties: globomapProperties{IPs: []string{"1.1.1.1"}}})
		}
		json.NewEncoder(w).Encode(
			struct{ Documents []globomapQueryResult }{
				Documents: queryResult,
			},
		)
	}))
	defer globomapApi.Close()
	os.Setenv("GLOBOMAP_API_HOSTNAME", globomapApi.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer close(requests)
		c.Assert(req.Method, check.Equals, http.MethodPost)
		c.Assert(req.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(req.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer req.Body.Close()
		c.Assert(data, check.HasLen, 1)

		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[0].Type, check.Equals, "edges")
		c.Assert(data[0].Key, check.Equals, "tsuru_1_1_1_1")
		c.Assert(el["id"], check.Equals, "1.1.1.1")
		c.Assert(el["name"], check.Equals, "node1")
		c.Assert(el["from"], check.Equals, "tsuru_pool/tsuru_pool1")
		c.Assert(el["to"], check.Equals, "comp_unit/globomap_node1")
		props, ok := el["properties"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(props["address"], check.Equals, "https://1.1.1.1:2376")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup([]string{"--repeat", "1m"})

	env.config.retrySleepTime = 0
	cmd := &updateCmd{}
	cmd.Run()

	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		c.Fail()
	}
}

func (s *S) TestUpdateCmdRunIgnoresFailedEvents(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/events":
			if req.FormValue("target.type") == "" {
				failedEvent := newEvent("app.delete", "myapp1")
				failedEvent.Error = "something wrong happened"
				events := []event{
					newEvent("app.create", "myapp1"),
					failedEvent,
				}
				json.NewEncoder(w).Encode(events)
			} else {
				json.NewEncoder(w).Encode(nil)
			}
		case "/1.0/apps/myapp1":
			json.NewEncoder(w).Encode(app{})
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 2)
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup(nil)

	cmd := &updateCmd{}
	cmd.Run()

	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		c.Fail()
	}
}
