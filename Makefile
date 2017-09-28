# Copyright 2017 tsuru authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

build:
	go build main.go tsuru_client.go globomap_client.go -o globomap-integration

run:
	go run main.go tsuru_client.go globomap_client.go

test:
	go test -check.v
