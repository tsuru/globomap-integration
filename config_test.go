// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"time"

	"gopkg.in/check.v1"
)

func (s *S) TestConfigDefault(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments(nil)
	c.Assert(err, check.IsNil)
	c.Assert(config.dry, check.Equals, false)
	c.Assert(config.verbose, check.Equals, false)
	c.Assert(config.start, check.NotNil)
	c.Assert(*config.start, check.DeepEquals, time.Duration(24*time.Hour))
	c.Assert(config.repeat, check.IsNil)
	c.Assert(config.retrySleepTime, check.Equals, 5*time.Minute)
	c.Assert(config.maxRetries, check.Equals, 20)
	c.Assert(env.cmd, check.FitsTypeOf, &updateCmd{})
}

func (s *S) TestConfigDry(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--dry"})
	c.Assert(err, check.IsNil)
	c.Assert(config.dry, check.Equals, true)
	c.Assert(env.cmd, check.FitsTypeOf, &updateCmd{})
}

func (s *S) TestConfigVerbose(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--verbose"})
	c.Assert(err, check.IsNil)
	c.Assert(config.verbose, check.Equals, true)
	c.Assert(env.cmd, check.FitsTypeOf, &updateCmd{})
}

func (s *S) TestConfigStartTime(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--start", "2d"})
	c.Assert(err, check.IsNil)
	c.Assert(config.start, check.NotNil)
	c.Assert(*config.start, check.DeepEquals, time.Duration(48*time.Hour))
	c.Assert(env.cmd, check.FitsTypeOf, &updateCmd{})
}

func (s *S) TestConfigInvalidStartTime(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--start", "invalid"})
	c.Assert(err, check.NotNil)
}

func (s *S) TestConfigRepeat(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--repeat", "20m"})
	c.Assert(err, check.IsNil)
	c.Assert(config.repeat, check.NotNil)
	c.Assert(*config.repeat, check.DeepEquals, time.Duration(20*time.Minute))
	c.Assert(config.start, check.NotNil)
	c.Assert(*config.start, check.DeepEquals, time.Duration(30*time.Minute))
	c.Assert(env.cmd, check.FitsTypeOf, &updateCmd{})

	err = config.ProcessArguments([]string{"--repeat", "3m"})
	c.Assert(err, check.IsNil)
	c.Assert(config.repeat, check.NotNil)
	c.Assert(*config.repeat, check.DeepEquals, time.Duration(3*time.Minute))
	c.Assert(config.start, check.NotNil)
	c.Assert(*config.start, check.DeepEquals, time.Duration(6*time.Minute))
}

func (s *S) TestConfigRetrySleepTime(c *check.C) {
	os.Setenv("RETRY_SLEEP_TIME", "2h")
	config := NewConfig()
	c.Assert(config.retrySleepTime, check.Equals, 2*time.Hour)

	os.Setenv("RETRY_SLEEP_TIME", "invalid")
	config = NewConfig()
	c.Assert(config.retrySleepTime, check.Equals, 5*time.Minute)
}

func (s *S) TestConfigMaxRetries(c *check.C) {
	os.Setenv("MAX_RETRIES", "5")
	config := NewConfig()
	c.Assert(config.maxRetries, check.Equals, 5)

	os.Setenv("MAX_RETRIES", "not a number")
	config = NewConfig()
	c.Assert(config.maxRetries, check.Equals, 20)
}

func (s *S) TestConfigInvalidRepeat(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--repeat", "foo"})
	c.Assert(err, check.NotNil)
}

func (s *S) TestConfigLoad(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--load"})
	c.Assert(err, check.IsNil)
	c.Assert(config.dry, check.Equals, false)
	c.Assert(config.verbose, check.Equals, false)
	c.Assert(env.cmd, check.FitsTypeOf, &loadCmd{})
}

func (s *S) TestConfigIncompatibleFlags(c *check.C) {
	config := NewConfig()
	err := config.ProcessArguments([]string{"--load", "--start", "2h"})
	c.Assert(err, check.NotNil)

	err = config.ProcessArguments([]string{"--load", "--repeat", "10m"})
	c.Assert(err, check.NotNil)

	err = config.ProcessArguments([]string{"--start", "1d", "--repeat", "30m"})
	c.Assert(err, check.NotNil)
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
