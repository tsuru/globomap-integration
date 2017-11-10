// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"sort"
	"testing"

	"gopkg.in/check.v1"
)

type S struct {
	token string
}

var _ = check.Suite(&S{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *S) SetUpSuite(c *check.C) {
	s.token = "mytoken"
}

func (s *S) SetUpTest(c *check.C) {
	os.Setenv("TSURU_HOSTNAME", "tsuru-host")
	os.Setenv("TSURU_TOKEN", s.token)
	os.Setenv("GLOBOMAP_LOADER_HOSTNAME", "globomap-loader")
	os.Setenv("GLOBOMAP_API_HOSTNAME", "globomap-api")
}

func (s *S) TearDownSuite(c *check.C) {
	os.Unsetenv("TSURU_HOSTNAME")
	os.Unsetenv("TSURU_TOKEN")
	os.Unsetenv("GLOBOMAP_LOADER_HOSTNAME")
	os.Unsetenv("GLOBOMAP_API_HOSTNAME")
}

func sortPayload(data []globomapPayload) {
	sort.Slice(data, func(i, j int) bool {
		collection1, _ := data[i]["collection"].(string)
		collection2, _ := data[j]["collection"].(string)
		if collection1 != collection2 {
			return collection1 < collection2
		}
		key1, _ := data[i]["key"].(string)
		key2, _ := data[j]["key"].(string)
		return key1 < key2
	})
}
