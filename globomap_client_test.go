// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"

	"gopkg.in/check.v1"
)

func (s *S) TestPost(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")
		c.Assert(r.Header.Get("Content-Type"), check.Equals, "application/json")

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(globomapResponse{Message: "ok"})
	}))
	defer server.Close()
	client := globomapClient{
		LoaderHostname: server.URL,
	}

	payload := []globomapPayload{
		map[string]interface{}{
			"k1": "v1",
			"k2": "v2",
		},
	}
	err := client.Post(payload)
	c.Assert(err, check.IsNil)
}

func (s *S) TestPostInChunks(c *check.C) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count := atomic.AddInt32(&requests, 1)
		c.Assert(req.Method, check.Equals, http.MethodPost)
		c.Assert(req.URL.Path, check.Equals, "/v1/updates")
		c.Assert(req.Header.Get("Content-Type"), check.Equals, "application/json")

		expectedPayloadLen := 100
		if count == 2 {
			expectedPayloadLen = 1
		}
		decoder := json.NewDecoder(req.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer req.Body.Close()
		c.Assert(data, check.HasLen, expectedPayloadLen)

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(globomapResponse{JobID: fmt.Sprintf("%d", count), Message: "ok"})
	}))
	defer server.Close()
	client := globomapClient{
		LoaderHostname: server.URL,
	}

	payload := make([]globomapPayload, 101)
	for i := 0; i <= 100; i++ {
		payload[i] = map[string]interface{}{
			fmt.Sprintf("k%d", i): fmt.Sprintf("v%d", i),
		}
	}
	env.config.sleepTimeBetweenChunks = 0
	err := client.Post(payload)
	c.Assert(err, check.IsNil)
	c.Assert(atomic.LoadInt32(&requests), check.Equals, int32(2))
}

func (s *S) TestPostInChunksWithErrors(c *check.C) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count := atomic.AddInt32(&requests, 1)
		c.Assert(req.Method, check.Equals, http.MethodPost)
		c.Assert(req.URL.Path, check.Equals, "/v1/updates")
		c.Assert(req.Header.Get("Content-Type"), check.Equals, "application/json")

		expectedPayloadLen := 100
		if count == 3 {
			expectedPayloadLen = 1
		}
		decoder := json.NewDecoder(req.Body)
		var data []globomapPayload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer req.Body.Close()
		c.Assert(data, check.HasLen, expectedPayloadLen)

		if count == 2 {
			json.NewEncoder(w).Encode(globomapResponse{JobID: fmt.Sprintf("%d", count), Message: "ok"})
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client := globomapClient{
		LoaderHostname: server.URL,
	}

	payload := make([]globomapPayload, 201)
	for i := 0; i <= 200; i++ {
		payload[i] = map[string]interface{}{
			fmt.Sprintf("k%d", i): fmt.Sprintf("v%d", i),
		}
	}

	env.config.sleepTimeBetweenChunks = 0
	err := client.Post(payload)
	c.Assert(err, check.NotNil)
	c.Assert(atomic.LoadInt32(&requests), check.Equals, int32(3))
}

func (s *S) TestPostNoContent(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.ExpectFailure("No request should have been done")
	}))
	defer server.Close()
	client := globomapClient{
		LoaderHostname: server.URL,
	}

	err := client.Post([]globomapPayload{})
	c.Assert(err, check.ErrorMatches, "No events to post")
}

func (s *S) TestGlobomapQuery(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c.Assert(req.Method, check.Equals, http.MethodGet)
		c.Assert(req.URL.Path, check.Equals, "/v1/collections/comp_unit/")
		query := req.FormValue("query")

		if strings.Contains(query, `"value":"vm-1234"`) {
			json.NewEncoder(w).Encode(
				struct{ Documents []globomapQueryResult }{
					[]globomapQueryResult{
						{Id: "abc", Name: "vm-1234", Properties: globomapProperties{IPs: []string{"10.52.20.20"}}},
						{Id: "def", Name: "vm-1234", Properties: globomapProperties{IPs: []string{"10.200.22.9"}}},
					},
				},
			)
		} else {
			json.NewEncoder(w).Encode(nil)
		}
	}))
	defer server.Close()
	client := globomapClient{
		ApiHostname: server.URL,
	}

	result, err := client.Query(globomapQueryFields{
		collection: "comp_unit",
		name:       "vm-1234",
		ip:         "10.200.22.9",
	})
	c.Assert(err, check.IsNil)
	c.Assert(result, check.NotNil)
	c.Assert(result.Id, check.Equals, "def")
	c.Assert(result.Name, check.Equals, "vm-1234")
	c.Assert(result.Properties.IPs, check.DeepEquals, []string{"10.200.22.9"})

	result, err = client.Query(globomapQueryFields{
		collection: "comp_unit",
		name:       "vm-123",
		ip:         "10.200.22.9",
	})
	c.Assert(err, check.IsNil)
	c.Assert(result, check.IsNil)
}

func (s *S) TestGlobomapQueryReturnsWhenOneResultWithoutIP(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodGet)
		c.Assert(r.URL.Path, check.Equals, "/v1/collections/comp_unit/")
		query := r.FormValue("query")
		c.Assert(strings.Contains(query, `"value":"vm-1234"`), check.Equals, true)

		json.NewEncoder(w).Encode(
			struct{ Documents []globomapQueryResult }{
				[]globomapQueryResult{{Id: "9876", Name: "vm-1234"}},
			},
		)
	}))
	defer server.Close()
	client := globomapClient{
		ApiHostname: server.URL,
	}

	result, err := client.Query(globomapQueryFields{
		collection: "comp_unit",
		name:       "vm-1234",
	})
	c.Assert(err, check.IsNil)
	c.Assert(result, check.NotNil)
	c.Assert(result.Id, check.Equals, "9876")
	c.Assert(result.Name, check.Equals, "vm-1234")
}

func (s *S) TestGlobomapResponseString(c *check.C) {
	r := globomapResponse{
		JobID:   "12345",
		Message: "Updates published successfully",
	}
	c.Assert(r.String(), check.Equals, "[12345] Updates published successfully")
}
