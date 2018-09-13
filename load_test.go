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
	"sync/atomic"
	"time"

	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
	"gopkg.in/check.v1"
)

func (s *S) TestLoadCmdRun(c *check.C) {
	var requestAppInfo1, requestAppInfo2 int32
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		a1 := app{Name: "myapp1", Pool: "pool1"}
		a2 := app{Name: "myapp2", Pool: "pool1"}
		services := []tsuru.Service{
			{Service: "myservice1", ServiceInstances: []tsuru.ServiceInstance{{ServiceName: "myservice1", Name: "myinstance"}}},
			{Service: "myservice2", ServiceInstances: []tsuru.ServiceInstance{{ServiceName: "myservice2", Name: "myinstance"}}},
		}
		switch req.URL.Path {
		case "/1.0/apps":
			json.NewEncoder(w).Encode([]app{a1, a2})
		case "/1.0/apps/myapp1":
			atomic.AddInt32(&requestAppInfo1, 1)
			a1.Description = "my first app"
			json.NewEncoder(w).Encode(a1)
		case "/1.0/apps/myapp2":
			atomic.AddInt32(&requestAppInfo2, 1)
			a2.Description = "my second app"
			json.NewEncoder(w).Encode(a2)
		case "/1.0/pools":
			json.NewEncoder(w).Encode([]pool{{Name: "pool1"}})
		case "/1.2/node":
			n1 := node{Pool: "pool1", Iaasid: "node1", Address: "https://1.1.1.1:2376"}
			n2 := node{Pool: "pool2", Iaasid: "node2", Address: "https://2.2.2.2:2376"}
			n3 := node{Pool: "pool1", Iaasid: "node3", Address: "https://3.3.3.3:2376"}
			json.NewEncoder(w).Encode(struct{ Nodes []node }{Nodes: []node{n1, n2, n3}})
		case "/1.0/services/instances":
			json.NewEncoder(w).Encode(services)
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	globomapApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c.Assert(req.Method, check.Equals, http.MethodGet)
		c.Assert(req.URL.Path, check.Equals, "/v1/collections/comp_unit/")
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
		}
		json.NewEncoder(w).Encode(
			struct{ Documents []globomapQueryResult }{
				Documents: queryResult,
			},
		)
	}))
	defer globomapApi.Close()
	os.Setenv("GLOBOMAP_API_HOSTNAME", globomapApi.URL)

	requests := make(chan bool, 5)
	globomapLoader := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			requests <- true
		}()
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()

		sortPayload(data)
		switch data[0].Collection {
		case "tsuru_app":
			c.Assert(data, check.HasLen, 4)

			el := data[0].Element
			c.Assert(data[0].Action, check.Equals, "UPDATE")
			c.Assert(data[0].Collection, check.Equals, "tsuru_app")
			c.Assert(data[0].Type, check.Equals, "collections")
			c.Assert(data[0].Key, check.Equals, "tsuru_myapp1")
			c.Assert(el["name"], check.Equals, "myapp1")
			props, ok := el["properties"].(map[string]interface{})
			c.Assert(ok, check.Equals, true)
			c.Assert(props["description"], check.Equals, "my first app")

			el = data[1].Element
			c.Assert(data[1].Action, check.Equals, "UPDATE")
			c.Assert(data[1].Collection, check.Equals, "tsuru_app")
			c.Assert(data[1].Type, check.Equals, "collections")
			c.Assert(data[1].Key, check.Equals, "tsuru_myapp2")
			c.Assert(el["name"], check.Equals, "myapp2")
			props, ok = el["properties"].(map[string]interface{})
			c.Assert(ok, check.Equals, true)
			c.Assert(props["description"], check.Equals, "my second app")

			el = data[2].Element
			c.Assert(ok, check.Equals, true)
			c.Assert(data[2].Action, check.Equals, "UPDATE")
			c.Assert(data[2].Collection, check.Equals, "tsuru_pool_app")
			c.Assert(data[2].Type, check.Equals, "edges")
			c.Assert(data[2].Key, check.Equals, "tsuru_myapp1-pool")
			c.Assert(el["name"], check.Equals, "myapp1-pool")
			c.Assert(el["from"], check.Equals, "tsuru_app/tsuru_myapp1")
			c.Assert(el["to"], check.Equals, "tsuru_pool/tsuru_pool1")

			el = data[3].Element
			c.Assert(ok, check.Equals, true)
			c.Assert(data[3].Action, check.Equals, "UPDATE")
			c.Assert(data[3].Collection, check.Equals, "tsuru_pool_app")
			c.Assert(data[3].Type, check.Equals, "edges")
			c.Assert(data[3].Key, check.Equals, "tsuru_myapp2-pool")
			c.Assert(el["name"], check.Equals, "myapp2-pool")
			c.Assert(el["from"], check.Equals, "tsuru_app/tsuru_myapp2")
			c.Assert(el["to"], check.Equals, "tsuru_pool/tsuru_pool1")

		case "tsuru_pool":
			c.Assert(data, check.HasLen, 1)
			el := data[0].Element
			c.Assert(data[0].Action, check.Equals, "UPDATE")
			c.Assert(data[0].Collection, check.Equals, "tsuru_pool")
			c.Assert(data[0].Type, check.Equals, "collections")
			c.Assert(data[0].Key, check.Equals, "tsuru_pool1")
			c.Assert(el["name"], check.Equals, "pool1")

		case "tsuru_pool_comp_unit":
			c.Assert(data, check.HasLen, 2)
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
		case "tsuru_service":
			c.Assert(data, check.HasLen, 2)
			c.Assert(data[0].Action, check.Equals, "UPDATE")
			c.Assert(data[0].Collection, check.Equals, "tsuru_service")
			c.Assert(data[0].Type, check.Equals, "collections")
			c.Assert(data[0].Key, check.Equals, "tsuru_myservice1")

			c.Assert(data[1].Action, check.Equals, "UPDATE")
			c.Assert(data[1].Collection, check.Equals, "tsuru_service")
			c.Assert(data[1].Type, check.Equals, "collections")
			c.Assert(data[1].Key, check.Equals, "tsuru_myservice2")
		case "tsuru_service_instance":
			c.Assert(data, check.HasLen, 2)
			c.Assert(data[0].Action, check.Equals, "UPDATE")
			c.Assert(data[0].Collection, check.Equals, "tsuru_service_instance")
			c.Assert(data[0].Type, check.Equals, "collections")
			c.Assert(data[0].Key, check.Equals, "tsuru_myservice1_myinstance")

			c.Assert(data[1].Action, check.Equals, "UPDATE")
			c.Assert(data[1].Collection, check.Equals, "tsuru_service_instance")
			c.Assert(data[1].Type, check.Equals, "collections")
			c.Assert(data[1].Key, check.Equals, "tsuru_myservice2_myinstance")
		}
	}))
	defer globomapLoader.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", globomapLoader.URL)
	setup(nil)

	cmd := &loadCmd{}
	cmd.Run()

	start := time.Now()
	fullTimeout := 5 * time.Second
	timeout := fullTimeout
	for i := 0; i < 3; i++ {
		select {
		case <-requests:
		case <-time.After(timeout):
			c.Fail()
		}
		timeout = fullTimeout - time.Since(start)
	}
	c.Assert(atomic.LoadInt32(&requestAppInfo1), check.Equals, int32(1))
	c.Assert(atomic.LoadInt32(&requestAppInfo2), check.Equals, int32(1))
}

func (s *S) TestLoadCmdRunNoRequestWhenNoData(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/1.0/apps":
			json.NewEncoder(w).Encode([]app{})
		case "/pools":
			json.NewEncoder(w).Encode([]pool{})
		case "/1.2/node":
			json.NewEncoder(w).Encode(nil)
		}
	}))
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOSTNAME", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.ExpectFailure("No request should have been done")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", server.URL)
	setup(nil)

	cmd := &loadCmd{}
	cmd.Run()

	select {
	case <-requests:
		c.Fail()
	case <-time.After(1 * time.Second):
	}
}

func (s *S) TestLoadCmdRunAppProperties(c *check.C) {
	tsuruServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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
		switch req.URL.Path {
		case "/1.0/apps":
			json.NewEncoder(w).Encode([]app{{Name: a.Name}})
		case "/1.0/apps/myapp1":
			json.NewEncoder(w).Encode(a)
		case "/pools":
			json.NewEncoder(w).Encode([]pool{})
		case "/1.2/node":
			json.NewEncoder(w).Encode(nil)
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

	cmd := &loadCmd{}
	cmd.Run()

	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		c.Fail()
	}
}
