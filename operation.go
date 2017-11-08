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
	toPayload() []globomapPayload
}

type tsuruOperation struct {
	action string
	time   time.Time
	target operationTarget
}

type nodeOperation struct {
	action   string
	time     time.Time
	nodeAddr string
}

type operationTarget interface {
	name() string
	collection() string
	toEdge(string) []globomapPayload
	properties() map[string]interface{}
}

type appOperation struct {
	appName   string
	cachedApp *app
}

type poolOperation struct {
	poolName string
}

func (op *tsuruOperation) toPayload() []globomapPayload {
	doc := op.toDocument()
	if doc == nil {
		return nil
	}
	payloads := []globomapPayload{*doc}
	edges := op.toEdge()
	if len(edges) > 0 {
		payloads = append(payloads, edges...)
	}
	return payloads
}

func (op *tsuruOperation) toDocument() *globomapPayload {
	props := op.baseDocument(op.target.name())
	if props == nil {
		return nil
	}

	(*props)["type"] = "collections"
	(*props)["collection"] = op.target.collection()

	return props
}

func (op *tsuruOperation) toEdge() []globomapPayload {
	doc := op.baseDocument(op.target.name())
	if doc == nil {
		return nil
	}

	edges := op.target.toEdge(op.action)
	if len(edges) == 0 {
		return nil
	}
	for _, edge := range edges {
		for k, v := range *doc {
			if _, ok := edge[k]; !ok {
				edge[k] = v
			}
		}
	}

	return edges
}

func (op *tsuruOperation) baseDocument(name string) *globomapPayload {
	action := op.action
	if action == "" {
		return nil
	}

	props := &globomapPayload{
		"action": action,
		"element": map[string]interface{}{
			"id":        name,
			"name":      name,
			"provider":  "tsuru",
			"timestamp": op.time.Unix(),
		},
		"key": "tsuru_" + name,
	}

	properties := map[string]interface{}{}
	propertiesMetadata := map[string]map[string]string{}
	if op.target == nil {
		return props
	}
	for k, v := range op.target.properties() {
		properties[k] = v
		propertiesMetadata[k] = map[string]string{
			"description": k,
		}
	}

	element, _ := (*props)["element"].(map[string]interface{})
	element["properties"] = properties
	element["properties_metadata"] = propertiesMetadata

	return props
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

func (op *appOperation) toEdge(action string) []globomapPayload {
	id := fmt.Sprintf("%s-pool", op.name())
	props := globomapPayload{
		"action":     action,
		"collection": "tsuru_pool_app",
		"type":       "edges",
		"element": map[string]interface{}{
			"id":        id,
			"name":      id,
			"provider":  "tsuru",
			"timestamp": time.Now().Unix(),
		},
		"key": "tsuru_" + id,
	}

	if props["action"] == "DELETE" {
		return []globomapPayload{props}
	}

	app, err := op.app()
	if err != nil {
		return nil
	}
	element, _ := props["element"].(map[string]interface{})
	element["from"] = "tsuru_app/tsuru_" + app.Name
	element["to"] = "tsuru_pool/tsuru_" + app.Pool
	return []globomapPayload{props}
}

func (op *appOperation) name() string {
	return op.appName
}

func (op *appOperation) collection() string {
	return "tsuru_app"
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

func (op *poolOperation) toEdge(action string) []globomapPayload {
	nodes, err := op.nodes()
	if err != nil || len(nodes) == 0 {
		return nil
	}
	edges := []globomapPayload{}
	for _, node := range nodes {
		id := fmt.Sprintf("%s-node", node.Name())
		edge := globomapPayload{
			"action":     action,
			"collection": "tsuru_pool_comp_unit",
			"type":       "edges",
			"element": map[string]interface{}{
				"id":        id,
				"name":      id,
				"provider":  "tsuru",
				"timestamp": time.Now().Unix(),
			},
			"key": "tsuru_" + id,
		}

		if edge["action"] == "DELETE" {
			edges = append(edges, edge)
			continue
		}

		element, _ := edge["element"].(map[string]interface{})
		element["from"] = "tsuru_pool/tsuru_" + op.poolName
		r, err := env.globomap.QueryByName("comp_unit", node.Name())
		if err != nil || len(r) != 1 {
			continue
		}
		element["to"] = r[0].Id

		element["properties"] = map[string]interface{}{
			"address": node.Addr(),
		}
		element["properties_metadata"] = map[string]map[string]string{
			"address": {"description": "address"},
		}

		edges = append(edges, edge)
	}
	return edges
}

func (op *poolOperation) name() string {
	return op.poolName
}

func (op *poolOperation) collection() string {
	return "tsuru_pool"
}

func (op *poolOperation) nodes() ([]node, error) {
	if len(env.nodes) == 0 {
		nodes, err := env.tsuru.NodeList()
		if err != nil {
			return nil, err
		}
		env.nodes = nodes
	}
	nodes := []node{}
	for _, node := range env.nodes {
		if node.Pool == op.poolName {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

func (op *nodeOperation) toPayload() []globomapPayload {
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
		return []globomapPayload{edge}
	}

	element, _ := edge["element"].(map[string]interface{})
	element["from"] = "tsuru_pool/tsuru_" + node.Pool
	r, err := env.globomap.QueryByName("comp_unit", node.Name())
	if err != nil || len(r) != 1 {
		return nil
	}
	element["to"] = r[0].Id

	element["properties"] = map[string]interface{}{
		"address": node.Addr(),
	}
	element["properties_metadata"] = map[string]map[string]string{
		"address": {"description": "address"},
	}

	return []globomapPayload{edge}
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

func NewTsuruOperation(events []event) *tsuruOperation {
	op := &tsuruOperation{
		action: "UPDATE",
		time:   time.Now(),
	}
	if len(events) == 0 {
		return op
	}
	op.time = events[len(events)-1].EndTime

	lastStatus := eventStatus(events[len(events)-1])
	if lastStatus == "CREATE" {
		lastStatus = "UPDATE"
	}
	op.action = lastStatus
	return op
}

func NewNodeOperation(events []event) *nodeOperation {
	op := &nodeOperation{
		action: "UPDATE",
		time:   time.Now(),
	}
	if len(events) == 0 {
		return op
	}
	op.time = events[len(events)-1].EndTime

	lastStatus := eventStatus(events[len(events)-1])
	if lastStatus == "CREATE" {
		lastStatus = "UPDATE"
	}
	op.action = lastStatus
	return op
}

func eventStatus(e event) string {
	parts := strings.Split(e.Kind.Name, ".")
	return strings.ToUpper(parts[1])
}
