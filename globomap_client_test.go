// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"gopkg.in/check.v1"
)

func (s *S) TestPost(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")
		c.Assert(r.Header.Get("Content-Type"), check.Equals, "application/json")

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

func (s *S) TestQueryByName(c *check.C) {
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

	results, err := client.QueryByName("comp_unit", "vm-1234")
	c.Assert(err, check.IsNil)
	c.Assert(results, check.HasLen, 1)
	c.Assert(results[0].Id, check.Equals, "9876")
	c.Assert(results[0].Name, check.Equals, "vm-1234")
}
