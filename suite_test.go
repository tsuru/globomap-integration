// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"sort"
	"testing"

	"github.com/tsuru/globomap-integration/globomap"
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

func sortPayload(data []globomap.Payload) {
	sort.Slice(data, func(i, j int) bool {
		if data[i].Collection != data[j].Collection {
			return data[i].Collection < data[j].Collection
		}
		return data[i].Key < data[j].Key
	})
}
