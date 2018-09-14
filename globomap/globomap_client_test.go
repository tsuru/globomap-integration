// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package globomap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gopkg.in/check.v1"
)

type S struct{}

var _ = check.Suite(&S{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *S) TestPost(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.URL.Path, check.Equals, "/v1/updates")
		c.Assert(r.Header.Get("Content-Type"), check.Equals, "application/json")

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response{Message: "ok"})
	}))
	defer server.Close()
	client := Client{
		LoaderHostname: server.URL,
	}

	payload := []Payload{{}}
	err := client.Post(payload)
	c.Assert(err, check.IsNil)
}

func (s *S) TestPostWithCredentials(c *check.C) {
	var auth, update bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, http.MethodPost)
		c.Assert(r.Header.Get("Content-Type"), check.Equals, "application/json")
		switch r.URL.Path {
		case "/v2/updates":
			update = true
			w.WriteHeader(http.StatusAccepted)
			c.Assert(r.Header.Get("x-driver-name"), check.Equals, "tsuru")
			c.Assert(r.Header.Get("Authorization"), check.Equals, "Token xpto")
			json.NewEncoder(w).Encode(response{Message: "ok"})
		case "/v2/auth":
			auth = true
			var credentials authRequest
			err := json.NewDecoder(r.Body).Decode(&credentials)
			c.Assert(credentials, check.DeepEquals, authRequest{Username: "user", Password: "password"})
			c.Assert(err, check.IsNil)
			json.NewEncoder(w).Encode(token{Token: "xpto"})
		default:
			c.Fatalf("Invalid request path called: %v", r.URL.Path)
		}
	}))
	defer server.Close()

	client := Client{
		LoaderHostname: server.URL,
		Username:       "user",
		Password:       "password",
	}

	payload := []Payload{{}}
	err := client.Post(payload)
	c.Assert(err, check.IsNil)
	c.Assert(auth, check.Equals, true)
	c.Assert(update, check.Equals, true)
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
		var data []Payload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer req.Body.Close()
		c.Assert(data, check.HasLen, expectedPayloadLen)

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response{JobID: fmt.Sprintf("%d", count), Message: "ok"})
	}))
	defer server.Close()
	client := Client{
		LoaderHostname: server.URL,
		ChunkInterval:  0,
	}

	payload := make([]Payload, 101)
	for i := 0; i <= 100; i++ {
		payload[i] = Payload{Key: fmt.Sprintf("k%d", i)}
	}
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
		var data []Payload
		err := decoder.Decode(&data)
		c.Assert(err, check.IsNil)
		defer req.Body.Close()
		c.Assert(data, check.HasLen, expectedPayloadLen)

		if count == 2 {
			json.NewEncoder(w).Encode(response{JobID: fmt.Sprintf("%d", count), Message: "ok"})
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client := Client{
		LoaderHostname: server.URL,
		ChunkInterval:  0,
	}

	payload := make([]Payload, 201)
	for i := 0; i <= 200; i++ {
		payload[i] = Payload{Key: fmt.Sprintf("k%d", i)}
	}

	err := client.Post(payload)
	c.Assert(err, check.NotNil)
	c.Assert(atomic.LoadInt32(&requests), check.Equals, int32(3))
}

func (s *S) TestPostNoContent(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.ExpectFailure("No request should have been done")
	}))
	defer server.Close()
	client := Client{
		LoaderHostname: server.URL,
	}

	err := client.Post([]Payload{})
	c.Assert(err, check.ErrorMatches, "No events to post")
}

func (s *S) TestGlobomapQuery(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c.Assert(req.Method, check.Equals, http.MethodGet)
		c.Assert(req.URL.Path, check.Equals, "/v1/collections/comp_unit/")
		query := req.FormValue("query")

		if strings.Contains(query, `"value":"vm-1234"`) {
			json.NewEncoder(w).Encode(
				struct{ Documents []QueryResult }{
					[]QueryResult{
						{Id: "abc", Name: "vm-1234", Properties: Properties{IPs: []string{"10.52.20.20"}}},
						{Id: "def", Name: "vm-1234", Properties: Properties{IPs: []string{"10.200.22.9"}}},
					},
				},
			)
		} else {
			json.NewEncoder(w).Encode(nil)
		}
	}))
	defer server.Close()
	client := Client{
		ApiHostname: server.URL,
	}

	result, err := client.Query(QueryFields{
		Collection: "comp_unit",
		Name:       "vm-1234",
		IP:         "10.200.22.9",
	})
	c.Assert(err, check.IsNil)
	c.Assert(result, check.NotNil)
	c.Assert(result.Id, check.Equals, "def")
	c.Assert(result.Name, check.Equals, "vm-1234")
	c.Assert(result.Properties.IPs, check.DeepEquals, []string{"10.200.22.9"})

	result, err = client.Query(QueryFields{
		Collection: "comp_unit",
		Name:       "vm-123",
		IP:         "10.200.22.9",
	})
	c.Assert(err, check.IsNil)
	c.Assert(result, check.IsNil)
}

func (s *S) TestGlobomapQueryWithCredentials(c *check.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v2/collections/comp_unit/":
			c.Assert(req.Method, check.Equals, http.MethodGet)
			query := req.FormValue("query")

			if strings.Contains(query, `"value":"vm-1234"`) {
				json.NewEncoder(w).Encode(
					struct{ Documents []QueryResult }{
						[]QueryResult{
							{Id: "abc", Name: "vm-1234", Properties: Properties{IPs: []string{"10.52.20.20"}}},
							{Id: "def", Name: "vm-1234", Properties: Properties{IPs: []string{"10.200.22.9"}}},
						},
					},
				)
			} else {
				json.NewEncoder(w).Encode(nil)
			}
		case "/v2/auth":
			var credentials authRequest
			err := json.NewDecoder(req.Body).Decode(&credentials)
			c.Assert(credentials, check.DeepEquals, authRequest{Username: "user", Password: "password"})
			c.Assert(err, check.IsNil)
			json.NewEncoder(w).Encode(token{Token: "xpto"})
		default:
			c.Fatalf("Invalid request path called: %v", req.URL.Path)
		}
	}))
	defer server.Close()
	client := Client{
		ApiHostname: server.URL,
		Username:    "user",
		Password:    "password",
	}

	result, err := client.Query(QueryFields{
		Collection: "comp_unit",
		Name:       "vm-1234",
		IP:         "10.200.22.9",
	})
	c.Assert(err, check.IsNil)
	c.Assert(result, check.NotNil)
	c.Assert(result.Id, check.Equals, "def")
	c.Assert(result.Name, check.Equals, "vm-1234")
	c.Assert(result.Properties.IPs, check.DeepEquals, []string{"10.200.22.9"})

	result, err = client.Query(QueryFields{
		Collection: "comp_unit",
		Name:       "vm-123",
		IP:         "10.200.22.9",
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
			struct{ Documents []QueryResult }{
				[]QueryResult{{Id: "9876", Name: "vm-1234"}},
			},
		)
	}))
	defer server.Close()
	client := Client{
		ApiHostname: server.URL,
	}

	result, err := client.Query(QueryFields{
		Collection: "comp_unit",
		Name:       "vm-1234",
	})
	c.Assert(err, check.IsNil)
	c.Assert(result, check.NotNil)
	c.Assert(result.Id, check.Equals, "9876")
	c.Assert(result.Name, check.Equals, "vm-1234")
}

func (s *S) TestGlobomapResponseString(c *check.C) {
	r := response{
		JobID:   "12345",
		Message: "Updates published successfully",
	}
	c.Assert(r.String(), check.Equals, "[12345] Updates published successfully")
}
