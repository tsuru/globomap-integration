// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"

	"gopkg.in/check.v1"
)

func (s *S) TestProcessEvents(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")

		decoder := json.NewDecoder(r.Body)
		var data map[string]interface{}
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer r.Body.Close()
		el, ok := data["element"].(map[string]interface{})
		c.Assert(ok, check.Equals, true)
		c.Assert(data["action"], check.Equals, "CREATE")
		c.Assert(data["collection"], check.Equals, "tsuru_app")
		c.Assert(el["name"], check.Equals, "myapp1")
	}))
	defer server.Close()
	os.Setenv("GLOBOMAP_HOSTNAME", server.URL)
	defer os.Unsetenv("GLOBOMAP_HOSTNAME")

	e1 := event{}
	e1.Target.Value = "myapp1"
	e1.Kind.Name = "app.create"
	processEvents([]event{e1})
}
