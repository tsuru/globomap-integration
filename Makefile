# Copyright 2017 tsuru authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

.PHONY: build test deploy

build:
	go build

test:
	go test -check.v -race

deploy:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o globomap-integration
	tsuru app-deploy Procfile globomap-integration -a ${APP}
