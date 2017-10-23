# Copyright 2017 tsuru authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

.PHONY: build run dry test

build:
	go build -o globomap-integration main.go config.go operation.go tsuru_client.go globomap_client.go

run:
	go run main.go config.go operation.go tsuru_client.go globomap_client.go

dry:
	go run main.go config.go operation.go tsuru_client.go globomap_client.go --dry

test:
	go test -check.v
