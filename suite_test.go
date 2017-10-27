// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
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
	os.Setenv("GLOBOMAP_HOSTNAME", "globomap-host")
}

func (s *S) TearDownSuite(c *check.C) {
	os.Unsetenv("TSURU_HOSTNAME")
	os.Unsetenv("TSURU_TOKEN")
	os.Unsetenv("GLOBOMAP_HOSTNAME")
}
