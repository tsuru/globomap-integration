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
	"sync"
	"sync/atomic"
	"time"

	"github.com/tsuru/globomap-integration/globomap"
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
	e.EndTime = time.Now()
	return e
}

type tsuruServer struct {
	*httptest.Server
	m             sync.Mutex
	appInfoCalled map[string]int
}

func newTsuruServer(events []event, services []tsuru.Service, apps []app, pools []pool, nodes []node) *tsuruServer {
	appIndex := make(map[string]app)
	for _, a := range apps {
		appIndex[a.Name] = a
	}
	tsuruServer := tsuruServer{
		appInfoCalled: make(map[string]int),
	}
	tsuruServer.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/1.0/apps/") {
			parts := strings.Split(req.URL.Path, "/")
			tsuruServer.m.Lock()
			defer tsuruServer.m.Unlock()
			app := parts[len(parts)-1]
			tsuruServer.appInfoCalled[app]++
			if app, ok := appIndex[app]; ok {
				json.NewEncoder(w).Encode(app)
				return
			}
		}
		switch req.URL.Path {
		case "/events":
			req.ParseForm()
			reqKinds := make(map[string]struct{})
			for _, k := range req.Form["kindname"] {
				reqKinds[k] = struct{}{}
			}
			var selEvents []event
			for _, e := range events {
				if _, ok := reqKinds[e.Kind.Name]; ok {
					selEvents = append(selEvents, e)
				}
			}
			json.NewEncoder(w).Encode(selEvents)
		case "/1.0/services/instances":
			json.NewEncoder(w).Encode(services)
		case "/1.0/pools":
			json.NewEncoder(w).Encode(pools)
		case "/1.2/node":
			json.NewEncoder(w).Encode(struct{ Nodes []node }{Nodes: nodes})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return &tsuruServer
}

func (s *S) TestUpdateCmdRun(c *check.C) {
	events := []event{
		newEvent("app.create", "myapp1"),
		newEvent("app.delete", "myapp2"),
		newEvent("pool.update", "pool1"),
		newEvent("pool.delete", "pool2"),
		newEvent("service.create", "service1"),
		newEvent("service.create", "service2"),
		newEvent("service.delete", "service2"),
		newEvent("service-instance.create", "service1/instance1"),
		newEvent("service-instance.create", "service1/instance2"),
		newEvent("service-instance.delete", "service1/instance1"),
	}
	bindEvent := newEvent("app.update.bind", "myapp1")
	b, err := bson.Marshal(&[]map[string]interface{}{
		{"name": ":service", "value": "service1"},
		{"name": ":instance", "value": "instance2"},
	})
	c.Assert(err, check.IsNil)
	bindEvent.StartCustomData = bson.Raw{Data: b, Kind: 4}
	bindEvent.EndTime = time.Now()
	events = append(events, bindEvent)
	services := []tsuru.Service{
		{
			Service: "service1",
			Plans:   []string{"small", "large"},
			ServiceInstances: []tsuru.ServiceInstance{
				{ServiceName: "service1", Name: "instance2"},
			},
		},
	}
	tsuruServer := newTsuruServer(events, services, []app{{Name: "myapp1", Pool: "pool1"}}, []pool{{Name: "pool1"}, {Name: "pool2"}}, nil)
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOST", tsuruServer.URL)
	requests := make(chan bool)
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomap.Payload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()

		sortPayload(data)

		v := atomic.AddInt32(&calls, 1)
		if v == 2 {
			el := data[0].Element
			c.Assert(data[0].Action, check.Equals, "UPDATE")
			c.Assert(data[0].Collection, check.Equals, "tsuru_app_service_instance")
			c.Assert(data[0].Type, check.Equals, globomap.PayloadTypeEdge)
			c.Assert(data[0].Key, check.Equals, "tsuru_myapp1_instance2")
			c.Assert(el["name"], check.Equals, "myapp1_instance2")
			c.Assert(el["from"], check.Equals, "tsuru_app/tsuru_myapp1")
			c.Assert(el["to"], check.Equals, "tsuru_service_instance/tsuru_service1_instance2")
			return
		}

		defer close(requests)

		c.Assert(len(data), check.Equals, 12)
		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_app")
		c.Assert(data[0].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[0].Key, check.Equals, "tsuru_myapp1")
		c.Assert(el["name"], check.Equals, "myapp1")

		c.Assert(data[1].Action, check.Equals, "DELETE")
		c.Assert(data[1].Collection, check.Equals, "tsuru_app")
		c.Assert(data[1].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[1].Key, check.Equals, "tsuru_myapp2")

		el = data[2].Element
		c.Assert(data[2].Action, check.Equals, "UPDATE")
		c.Assert(data[2].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[2].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[2].Key, check.Equals, "tsuru_pool1")
		c.Assert(el["name"], check.Equals, "pool1")

		c.Assert(data[3].Action, check.Equals, "DELETE")
		c.Assert(data[3].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[3].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[3].Key, check.Equals, "tsuru_pool2")

		el = data[4].Element
		c.Assert(data[4].Action, check.Equals, "UPDATE")
		c.Assert(data[4].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[4].Type, check.Equals, globomap.PayloadTypeEdge)
		c.Assert(data[4].Key, check.Equals, "tsuru_myapp1-pool")
		c.Assert(el["name"], check.Equals, "myapp1-pool")
		c.Assert(el["from"], check.Equals, "tsuru_app/tsuru_myapp1")
		c.Assert(el["to"], check.Equals, "tsuru_pool/tsuru_pool1")

		c.Assert(data[5].Action, check.Equals, "DELETE")
		c.Assert(data[5].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[5].Type, check.Equals, globomap.PayloadTypeEdge)
		c.Assert(data[5].Key, check.Equals, "tsuru_myapp2-pool")

		c.Assert(data[6].Action, check.Equals, "UPDATE")
		c.Assert(data[6].Collection, check.Equals, "tsuru_service")
		c.Assert(data[6].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[6].Key, check.Equals, "tsuru_service1")

		c.Assert(data[7].Action, check.Equals, "DELETE")
		c.Assert(data[7].Collection, check.Equals, "tsuru_service")
		c.Assert(data[7].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[7].Key, check.Equals, "tsuru_service2")

		c.Assert(data[8].Action, check.Equals, "DELETE")
		c.Assert(data[8].Collection, check.Equals, "tsuru_service_instance")
		c.Assert(data[8].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[8].Key, check.Equals, "tsuru_service1_instance1")

		c.Assert(data[9].Action, check.Equals, "UPDATE")
		c.Assert(data[9].Collection, check.Equals, "tsuru_service_instance")
		c.Assert(data[9].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[9].Key, check.Equals, "tsuru_service1_instance2")

		c.Assert(data[10].Action, check.Equals, "DELETE")
		c.Assert(data[10].Collection, check.Equals, "tsuru_service_service_instance")
		c.Assert(data[10].Type, check.Equals, globomap.PayloadTypeEdge)
		c.Assert(data[10].Key, check.Equals, "tsuru_service1_instance1")

		c.Assert(data[11].Action, check.Equals, "UPDATE")
		c.Assert(data[11].Collection, check.Equals, "tsuru_service_service_instance")
		c.Assert(data[11].Type, check.Equals, globomap.PayloadTypeEdge)
		c.Assert(data[11].Key, check.Equals, "tsuru_service1_instance2")
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
	c.Assert(calls, check.Equals, int32(2))
}

func (s *S) TestUpdateCmdRunWithMultipleEventsPerKind(c *check.C) {
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
	apps := []app{{Name: "myapp1", Pool: "pool1"}, {Name: "myapp2", Pool: "pool1"}}
	pools := []pool{{Name: "pool1"}}
	tsuruServer := newTsuruServer(events, nil, apps, pools, nil)
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOST", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomap.Payload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 6)

		sortPayload(data)
		c.Assert(data[0].Action, check.Equals, "DELETE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_app")
		c.Assert(data[0].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[0].Key, check.Equals, "tsuru_myapp1")

		el := data[1].Element
		c.Assert(data[1].Action, check.Equals, "UPDATE")
		c.Assert(data[1].Collection, check.Equals, "tsuru_app")
		c.Assert(data[1].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[1].Key, check.Equals, "tsuru_myapp2")
		c.Assert(el["name"], check.Equals, "myapp2")

		el = data[2].Element
		c.Assert(data[2].Action, check.Equals, "UPDATE")
		c.Assert(data[2].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[2].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[2].Key, check.Equals, "tsuru_pool1")
		c.Assert(el["name"], check.Equals, "pool1")

		c.Assert(data[3].Action, check.Equals, "DELETE")
		c.Assert(data[3].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[3].Type, check.Equals, globomap.PayloadTypeCollection)
		c.Assert(data[3].Key, check.Equals, "tsuru_pool2")

		c.Assert(data[4].Action, check.Equals, "DELETE")
		c.Assert(data[4].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[4].Type, check.Equals, globomap.PayloadTypeEdge)
		c.Assert(data[4].Key, check.Equals, "tsuru_myapp1-pool")

		el = data[5].Element
		c.Assert(data[5].Action, check.Equals, "UPDATE")
		c.Assert(data[5].Collection, check.Equals, "tsuru_pool_app")
		c.Assert(data[5].Type, check.Equals, globomap.PayloadTypeEdge)
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
	c.Assert(tsuruServer.appInfoCalled["myapp1"], check.Equals, 1)
	c.Assert(tsuruServer.appInfoCalled["myapp2"], check.Equals, 1)
}

func (s *S) TestUpdateCmdRunNoRequestWhenNoEventsToPost(c *check.C) {
	tsuruServer := newTsuruServer(nil, nil, nil, nil, nil)
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOST", tsuruServer.URL)

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
	tsuruServer := newTsuruServer([]event{newEvent("app.create", "myapp1")}, nil, []app{a}, nil, nil)
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOST", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomap.Payload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 2)

		sortPayload(data)
		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_app")
		c.Assert(data[0].Type, check.Equals, globomap.PayloadTypeCollection)
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
		c.Assert(data[1].Type, check.Equals, globomap.PayloadTypeEdge)
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
	tsuruServer := newTsuruServer([]event{
		newEvent("pool.create", "pool1"),
	}, nil, nil, []pool{pool{
		Name:        "pool1",
		Provisioner: "docker",
		Default_:    false,
		Public:      true,
		Teams:       []string{"team1", "team2", "team3"},
	}}, nil)
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOST", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomap.Payload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		c.Assert(data, check.HasLen, 1)

		sortPayload(data)
		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_pool")
		c.Assert(data[0].Type, check.Equals, globomap.PayloadTypeCollection)
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
	healing := event{}
	events := []event{
		newEvent("node.create", "1.1.1.1"),
		newEvent("node.delete", "https://2.2.2.2:2376"),
		newEvent("node.create", "3.3.3.3"),
		newEvent("node.create", "https://4.4.4.4:2376"),
	}
	healing.Target.Type = "node"
	healing.Target.Value = "https://4.4.4.4:2376"
	healing.Kind.Name = "healer"
	data := struct {
		Id string `bson:"_id"`
	}{"https://5.5.5.5:2376"}
	b, err := bson.Marshal(data)
	c.Assert(err, check.IsNil)
	healing.EndCustomData = bson.Raw{Data: b, Kind: 3}
	healing.EndTime = time.Now()
	events = append(events, healing)
	pools := []pool{{
		Name:        "pool1",
		Provisioner: "docker",
		Default_:    false,
		Public:      true,
		Teams:       []string{"team1", "team2", "team3"},
	}, {
		Name:        "pool2",
		Provisioner: "swarm",
		Default_:    false,
		Public:      false,
		Teams:       []string{"team1"},
	}}
	nodes := []node{
		{Pool: "pool1", Iaasid: "node1", Address: "https://1.1.1.1:2376"},
		{Pool: "pool2", Iaasid: "node5", Address: "https://5.5.5.5:2376"},
		{Pool: "pool2", Iaasid: "node3", Address: "3.3.3.3"},
	}
	tsuruServer := newTsuruServer(events, nil, nil, pools, nodes)
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOST", tsuruServer.URL)

	globomapApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		re := regexp.MustCompile(`"value":"([^"]*)"`)
		matches := re.FindAllStringSubmatch(req.FormValue("query"), -1)
		c.Assert(matches, check.HasLen, 1)
		c.Assert(matches[0], check.HasLen, 2)

		name := matches[0][1]
		queryResult := []globomap.QueryResult{}
		switch name {
		case "node1":
			queryResult = append(queryResult, globomap.QueryResult{Id: "comp_unit/globomap_node1", Name: "node1", Properties: globomap.Properties{IPs: []string{"1.1.1.1"}}})
		case "node3":
			queryResult = append(queryResult, globomap.QueryResult{Id: "comp_unit/globomap_node3", Name: "node3", Properties: globomap.Properties{IPs: []string{"3.3.3.3"}}})
		case "node5":
			queryResult = append(queryResult, globomap.QueryResult{Id: "comp_unit/globomap_node5", Name: "node5", Properties: globomap.Properties{IPs: []string{"5.5.5.5"}}})
		}
		json.NewEncoder(w).Encode(
			struct{ Documents []globomap.QueryResult }{
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
		var data []globomap.Payload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer req.Body.Close()

		c.Assert(data, check.HasLen, 5)

		sortPayload(data)
		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[0].Type, check.Equals, globomap.PayloadTypeEdge)
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
		c.Assert(data[1].Type, check.Equals, globomap.PayloadTypeEdge)
		c.Assert(data[1].Key, check.Equals, "tsuru_2_2_2_2")

		el = data[2].Element
		c.Assert(data[2].Action, check.Equals, "UPDATE")
		c.Assert(data[2].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[2].Type, check.Equals, globomap.PayloadTypeEdge)
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
		c.Assert(data[3].Type, check.Equals, globomap.PayloadTypeEdge)
		c.Assert(data[3].Key, check.Equals, "tsuru_4_4_4_4")

		el = data[4].Element
		c.Assert(data[4].Action, check.Equals, "UPDATE")
		c.Assert(data[4].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[4].Type, check.Equals, globomap.PayloadTypeEdge)
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
	tsuruServer := newTsuruServer([]event{newEvent("node.create", "1.1.1.1")}, nil, nil, []pool{{
		Name:        "pool1",
		Provisioner: "docker",
		Default_:    false,
		Public:      true,
		Teams:       []string{"team1", "team2", "team3"},
	}}, []node{{Pool: "pool1", Iaasid: "node1", Address: "https://1.1.1.1:2376"}})
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOST", tsuruServer.URL)

	var globomapApiRequests int32
	globomapApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		re := regexp.MustCompile(`"value":"([^"]*)"`)
		matches := re.FindAllStringSubmatch(req.FormValue("query"), -1)
		c.Assert(matches, check.HasLen, 1)
		c.Assert(matches[0], check.HasLen, 2)

		queryResult := []globomap.QueryResult{}
		if atomic.AddInt32(&globomapApiRequests, 1) > 1 {
			queryResult = append(queryResult, globomap.QueryResult{Id: "comp_unit/globomap_node1", Name: "node1", Properties: globomap.Properties{IPs: []string{"1.1.1.1"}}})
		}
		json.NewEncoder(w).Encode(
			struct{ Documents []globomap.QueryResult }{
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
		var data []globomap.Payload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer req.Body.Close()
		c.Assert(data, check.HasLen, 1)

		el := data[0].Element
		c.Assert(data[0].Action, check.Equals, "UPDATE")
		c.Assert(data[0].Collection, check.Equals, "tsuru_pool_comp_unit")
		c.Assert(data[0].Type, check.Equals, globomap.PayloadTypeEdge)
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
	failedEvent := newEvent("app.delete", "myapp1")
	failedEvent.Error = "something wrong happened"
	events := []event{
		newEvent("app.create", "myapp1"),
		failedEvent,
	}
	tsuruServer := newTsuruServer(events, nil, []app{{Name: "myapp1"}}, nil, nil)
	defer tsuruServer.Close()
	os.Setenv("TSURU_HOST", tsuruServer.URL)

	requests := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requests)
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data []globomap.Payload
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
