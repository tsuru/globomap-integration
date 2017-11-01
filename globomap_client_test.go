// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

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
