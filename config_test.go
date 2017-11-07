// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"gopkg.in/check.v1"
)

func (s *S) TestConfigDefault(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments(nil)
	c.Assert(err, check.IsNil)
	c.Assert(config.dry, check.Equals, false)
	c.Assert(env.cmd, check.FitsTypeOf, &updateCmd{})
}

func (s *S) TestConfigDry(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--dry"})
	c.Assert(err, check.IsNil)
	c.Assert(config.dry, check.Equals, true)
	c.Assert(env.cmd, check.FitsTypeOf, &updateCmd{})
}

func (s *S) TestConfigStartTime(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--start", "2d"})
	c.Assert(err, check.IsNil)
	c.Assert(config.dry, check.Equals, false)
	c.Assert(env.cmd, check.FitsTypeOf, &updateCmd{})
}

func (s *S) TestConfigInvalidStartTime(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--start", "invalid"})
	c.Assert(err, check.NotNil)
}

func (s *S) TestConfigLoad(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--load"})
	c.Assert(err, check.IsNil)
	c.Assert(config.dry, check.Equals, false)
	c.Assert(env.cmd, check.FitsTypeOf, &loadCmd{})
}

func (s *S) TestConfigMissingEnvVars(c *check.C) {
	config := NewConfig()
	config.tsuruHostname = ""
	err := config.ProcessArguments(nil)
	c.Assert(err, check.NotNil)

	config.tsuruHostname = "host"
	config.tsuruToken = ""
	err = config.ProcessArguments(nil)
	c.Assert(err, check.NotNil)

	config.tsuruToken = "token"
	config.globomapApiHostname = ""
	err = config.ProcessArguments(nil)
	c.Assert(err, check.NotNil)

	config.globomapApiHostname = "host"
	config.globomapLoaderHostname = ""
	err = config.ProcessArguments(nil)
	c.Assert(err, check.NotNil)

	err = config.ProcessArguments([]string{"--dry"})
	c.Assert(err, check.IsNil)
}
