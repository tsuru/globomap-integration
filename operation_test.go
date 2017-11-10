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

	check "gopkg.in/check.v1"
)

func (s *S) TestNodeOperationNode(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.URL.Path, check.Equals, "/node")

		n1 := node{Id: "node1", Address: "https://10.20.30.40:2376"}
		n2 := node{Id: "node2", Address: "https://10.20.30.41:2376"}
		n3 := node{Id: "node3", Address: "https://10.20.30.42:2376"}
		json.NewEncoder(w).Encode(struct{ Nodes []node }{Nodes: []node{n1, n2, n3}})
	}))
	defer server.Close()
	os.Setenv("TSURU_HOSTNAME", server.URL)
	setup(nil)

	op := &nodeOperation{nodeAddr: "https://10.20.30.41:2376"}
	node, err := op.node()
	c.Assert(err, check.IsNil)
	c.Assert(node, check.NotNil)
	c.Assert(node.Id, check.Equals, "node2")
}

func (s *S) TestNodeOperationNodeCacheRequest(c *check.C) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		c.Assert(r.URL.Path, check.Equals, "/node")

		n1 := node{Id: "node1", Address: "https://10.20.30.40:2376"}
		json.NewEncoder(w).Encode(struct{ Nodes []node }{Nodes: []node{n1}})
	}))
	defer server.Close()
	os.Setenv("TSURU_HOSTNAME", server.URL)
	setup(nil)

	op := &nodeOperation{nodeAddr: "https://10.20.30.40:2376"}
	op.node()
	op.node()
	c.Assert(atomic.LoadInt32(&requests), check.Equals, int32(1))
}

func (s *S) TestNodeOperationNodeError(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.URL.Path, check.Equals, "/node")

		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	os.Setenv("TSURU_HOSTNAME", server.URL)
	setup(nil)

	op := &nodeOperation{}
	node, err := op.node()
	c.Assert(err, check.NotNil)
	c.Assert(node, check.IsNil)
}

func (s *S) TestNodeOperationNodeIP(c *check.C) {
	op := &nodeOperation{}
	c.Assert(op.nodeIP(), check.Equals, "")

	op.nodeAddr = "10.20.11.113"
	c.Assert(op.nodeIP(), check.Equals, "10.20.11.113")

	op.nodeAddr = "https://200.53.19.88:2376"
	c.Assert(op.nodeIP(), check.Equals, "200.53.19.88")

	op.nodeAddr = "invalid"
	c.Assert(op.nodeIP(), check.Equals, "")
}
