// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type operation interface {
	toPayload() *globomapPayload
}

type nodeOperation struct {
	action   string
	time     time.Time
	nodeAddr string
}

type appOperation struct {
	action    string
	time      time.Time
	appName   string
	cachedApp *app
}

type appPoolOperation struct {
	action    string
	time      time.Time
	appName   string
	cachedApp *app
}

type poolOperation struct {
	action   string
	time     time.Time
	poolName string
}

var (
	_ operation = &nodeOperation{}
	_ operation = &appPoolOperation{}
	_ operation = &appOperation{}
	_ operation = &poolOperation{}
)

func eventStatus(e event) string {
	parts := strings.Split(e.Kind.Name, ".")
	status := strings.ToUpper(parts[1])
	if status == "CREATE" {
		status = "UPDATE"
	}
	return status
}

func baseDocument(name, action, collection string, time time.Time, props map[string]interface{}) *globomapPayload {
	doc := &globomapPayload{
		"action":     action,
		"collection": collection,
		"element": map[string]interface{}{
			"id":        name,
			"name":      name,
			"provider":  "tsuru",
			"timestamp": time.Unix(),
		},
		"key":  "tsuru_" + name,
		"type": "collections",
	}

	properties := map[string]interface{}{}
	propertiesMetadata := map[string]map[string]string{}
	for k, v := range props {
		properties[k] = v
		propertiesMetadata[k] = map[string]string{
			"description": k,
		}
	}

	element, _ := (*doc)["element"].(map[string]interface{})
	element["properties"] = properties
	element["properties_metadata"] = propertiesMetadata

	return doc
}

func (op *appOperation) toPayload() *globomapPayload {
	return baseDocument(op.appName, op.action, "tsuru_app", op.time, op.properties())
}

func (op *appOperation) app() (*app, error) {
	var err error
	if op.cachedApp == nil {
		op.cachedApp, err = env.tsuru.AppInfo(op.appName)
	}
	return op.cachedApp, err
}

func (op *appOperation) properties() map[string]interface{} {
	app, _ := op.app()
	if app == nil {
		return nil
	}

	return map[string]interface{}{
		"description":   app.Description,
		"tags":          app.Tags,
		"platform":      app.Platform,
		"addresses":     app.Addresses(),
		"router":        app.Router,
		"owner":         app.Owner,
		"team_owner":    app.TeamOwner,
		"teams":         app.Teams,
		"plan_name":     app.Plan.Name,
		"plan_router":   app.Plan.Router,
		"plan_memory":   strconv.Itoa(app.Plan.Memory),
		"plan_swap":     strconv.Itoa(app.Plan.Swap),
		"plan_cpushare": strconv.Itoa(app.Plan.Cpushare),
	}
}

func (op *appPoolOperation) app() (*app, error) {
	var err error
	if op.cachedApp == nil {
		op.cachedApp, err = env.tsuru.AppInfo(op.appName)
	}
	return op.cachedApp, err
}

func (op *appPoolOperation) toPayload() *globomapPayload {
	id := fmt.Sprintf("%s-pool", op.appName)
	props := globomapPayload{
		"action":     op.action,
		"collection": "tsuru_pool_app",
		"type":       "edges",
		"element": map[string]interface{}{
			"id":        id,
			"name":      id,
			"provider":  "tsuru",
			"timestamp": op.time.Unix(),
		},
		"key": "tsuru_" + id,
	}

	if props["action"] == "DELETE" {
		return &props
	}

	app, err := op.app()
	if err != nil {
		return nil
	}
	element, _ := props["element"].(map[string]interface{})
	element["from"] = "tsuru_app/tsuru_" + app.Name
	element["to"] = "tsuru_pool/tsuru_" + app.Pool
	return &props
}

func (op *poolOperation) toPayload() *globomapPayload {
	return baseDocument(op.poolName, op.action, "tsuru_pool", op.time, op.properties())
}

func (op *poolOperation) pool() *pool {
	for _, p := range env.pools {
		if p.Name == op.poolName {
			return &p
		}
	}
	return nil
}

func (op *poolOperation) properties() map[string]interface{} {
	pool := op.pool()
	if pool == nil {
		return nil
	}

	return map[string]interface{}{
		"provisioner": pool.Provisioner,
		"default":     strconv.FormatBool(pool.Default),
		"public":      strconv.FormatBool(pool.Public),
		"teams":       pool.Teams,
	}
}

func (op *nodeOperation) toPayload() *globomapPayload {
	node, err := op.node()
	if err != nil || node == nil {
		return nil
	}

	id := fmt.Sprintf("%s-node", node.Name())
	edge := globomapPayload{
		"action":     op.action,
		"collection": "tsuru_pool_comp_unit",
		"type":       "edges",
		"element": map[string]interface{}{
			"id":        id,
			"name":      id,
			"provider":  "tsuru",
			"timestamp": op.time.Unix(),
		},
		"key": "tsuru_" + id,
	}

	if edge["action"] == "DELETE" {
		return &edge
	}

	element, _ := edge["element"].(map[string]interface{})
	element["from"] = "tsuru_pool/tsuru_" + node.Pool
	r, err := env.globomap.QueryByName("comp_unit", node.Name())
	if err != nil || len(r) != 1 {
		if env.config.verbose {
			fmt.Printf("node %s not found in globomap API\n", node.Name())
		}
		return nil
	}
	element["to"] = r[0].Id

	element["properties"] = map[string]interface{}{
		"address": node.Addr(),
	}
	element["properties_metadata"] = map[string]map[string]string{
		"address": {"description": "address"},
	}

	return &edge
}

func (op *nodeOperation) node() (*node, error) {
	if len(env.nodes) == 0 {
		nodes, err := env.tsuru.NodeList()
		if err != nil {
			return nil, err
		}
		env.nodes = nodes
	}
	for _, node := range env.nodes {
		if node.Addr() == op.nodeAddr {
			return &node, nil
		}
	}

	return nil, nil
}
