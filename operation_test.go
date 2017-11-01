// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"

	"gopkg.in/check.v1"
)

func (s *S) TestPoolOperationNodes(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.URL.Path, check.Equals, "/node")

		n1 := node{Id: "node1", Pool: "prod"}
		n2 := node{Id: "node2", Pool: "dev"}
		n3 := node{Id: "node3", Pool: "dev"}
		json.NewEncoder(w).Encode(struct{ Nodes []node }{Nodes: []node{n1, n2, n3}})
	}))
	defer server.Close()
	os.Setenv("TSURU_HOSTNAME", server.URL)
	setup(nil)

	op := &poolOperation{poolName: "dev"}
	nodes, err := op.nodes()
	c.Assert(err, check.IsNil)
	c.Assert(nodes, check.HasLen, 2)
	c.Assert(nodes[0].Id, check.Equals, "node2")
	c.Assert(nodes[1].Id, check.Equals, "node3")
}

func (s *S) TestPoolOperationNodesCacheRequest(c *check.C) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		c.Assert(r.URL.Path, check.Equals, "/node")

		n1 := node{Id: "node1", Pool: "prod"}
		json.NewEncoder(w).Encode(struct{ Nodes []node }{Nodes: []node{n1}})
	}))
	defer server.Close()
	os.Setenv("TSURU_HOSTNAME", server.URL)
	setup(nil)

	op := &poolOperation{poolName: "dev"}
	op.nodes()
	op.nodes()
	c.Assert(atomic.LoadInt32(&requests), check.Equals, int32(1))
}

func (s *S) TestPoolOperationNodesError(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.URL.Path, check.Equals, "/node")

		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	os.Setenv("TSURU_HOSTNAME", server.URL)
	setup(nil)

	op := &poolOperation{poolName: "dev"}
	nodes, err := op.nodes()
	c.Assert(err, check.NotNil)
	c.Assert(nodes, check.IsNil)
}
