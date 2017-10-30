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

type operation struct {
	action string
	time   time.Time
	target operationTarget
}

type operationTarget interface {
	name() string
	collection() string
	toEdge(string) *globomapPayload
	properties() map[string]string
}

type appOperation struct {
	appName   string
	cachedApp *app
}

type poolOperation struct {
	poolName string
}

func (op *operation) toPayload() []globomapPayload {
	doc := op.toDocument()
	if doc == nil {
		return nil
	}
	payloads := []globomapPayload{*doc}
	edge := op.toEdge()
	if edge != nil {
		payloads = append(payloads, *edge)
	}
	return payloads
}

func (op *operation) toDocument() *globomapPayload {
	props := op.baseDocument(op.target.name())
	if props == nil {
		return nil
	}

	(*props)["type"] = "collections"
	(*props)["collection"] = op.target.collection()

	return props
}

func (op *operation) toEdge() *globomapPayload {
	doc := op.baseDocument(op.target.name())
	if doc == nil {
		return nil
	}

	edge := op.target.toEdge(op.action)
	if edge == nil {
		return nil
	}
	for k, v := range *doc {
		if _, ok := (*edge)[k]; !ok {
			(*edge)[k] = v
		}
	}

	return edge
}

func (op *operation) baseDocument(name string) *globomapPayload {
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
	}

	if action != "CREATE" {
		(*props)["key"] = "tsuru_" + name
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

func (op *appOperation) properties() map[string]string {
	app, _ := op.app()
	if app == nil {
		return nil
	}

	return map[string]string{
		"description":   app.Description,
		"tags":          strings.Join(app.Tags, ", "),
		"platform":      app.Platform,
		"addresses":     strings.Join(app.Addresses(), ", "),
		"router":        app.Router,
		"owner":         app.Owner,
		"team_owner":    app.TeamOwner,
		"teams":         strings.Join(app.Teams, ", "),
		"plan_name":     app.Plan.Name,
		"plan_router":   app.Plan.Router,
		"plan_memory":   strconv.Itoa(app.Plan.Memory),
		"plan_swap":     strconv.Itoa(app.Plan.Swap),
		"plan_cpushare": strconv.Itoa(app.Plan.Cpushare),
	}
}

func (op *appOperation) toEdge(action string) *globomapPayload {
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
	}

	if props["action"] != "CREATE" {
		props["key"] = "tsuru_" + id
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

func (op *poolOperation) properties() map[string]string {
	pool := op.pool()
	if pool == nil {
		return nil
	}

	return map[string]string{
		"provisioner": pool.Provisioner,
		"default":     strconv.FormatBool(pool.Default),
		"public":      strconv.FormatBool(pool.Public),
		"Teams":       strings.Join(pool.Teams, ", "),
	}
}

func (op *poolOperation) toEdge(action string) *globomapPayload {
	return nil
}

func (op *poolOperation) name() string {
	return op.poolName
}

func (op *poolOperation) collection() string {
	return "tsuru_pool"
}

func NewOperation(events []event) operation {
	op := operation{
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
