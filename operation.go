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
	name       string
	collection string
	events     []event
	target     operationTarget
}

type operationTarget interface {
	toEdge() *globomapPayload
	properties() map[string]string
}

type appOperation struct {
	cachedApp *app
	parent    operation
}

type poolOperation struct {
	parent operation
}

func (op *operation) Time() time.Time {
	if len(op.events) > 0 {
		return op.events[len(op.events)-1].EndTime
	}
	return time.Now()
}

func (op *operation) action() string {
	firstStatus := eventStatus(op.events[0])
	if len(op.events) == 1 {
		return firstStatus
	}

	lastStatus := eventStatus(op.events[len(op.events)-1])
	if lastStatus == "DELETE" {
		if firstStatus == "CREATE" {
			return "" // nothing to do
		}
		return "DELETE"
	}
	return firstStatus
}

func (op *appOperation) app() (*app, error) {
	var err error
	if op.cachedApp == nil {
		op.cachedApp, err = tsuru.AppInfo(op.parent.name)
	}
	return op.cachedApp, err
}

func (op *poolOperation) pool() *pool {
	for _, p := range pools {
		if p.Name == op.parent.name {
			return &p
		}
	}
	return nil
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

func (op *operation) toPayload() []globomapPayload {
	doc := op.toDocument()
	if doc == nil {
		return nil
	}
	payloads := []globomapPayload{*doc}
	edge := op.target.toEdge()
	if edge != nil {
		payloads = append(payloads, *edge)
	}
	return payloads
}

func (op *operation) toDocument() *globomapPayload {
	action := op.action()
	if action == "" {
		return nil
	}

	props := &globomapPayload{
		"action":     action,
		"type":       "collections",
		"collection": op.collection,
		"element": map[string]interface{}{
			"id":        op.name,
			"name":      op.name,
			"provider":  "tsuru",
			"timestamp": op.Time().Unix(),
		},
	}

	if action != "CREATE" {
		(*props)["key"] = "tsuru_" + op.name
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

func (op *appOperation) toEdge() *globomapPayload {
	props := op.parent.toDocument()
	if props == nil {
		return nil
	}

	doc := *props
	doc["collection"] = "tsuru_pool_app"
	doc["type"] = "edges"
	id := fmt.Sprintf("%s-pool", op.parent.name)
	if doc["action"] != "CREATE" {
		doc["key"] = "tsuru_" + id
	}
	element, _ := doc["element"].(map[string]interface{})
	element["id"] = id
	element["name"] = id
	delete(element, "properties")
	delete(element, "properties_metadata")
	if doc["action"] == "DELETE" {
		return props
	}

	app, err := op.app()
	if err != nil {
		return nil
	}
	element["from"] = "tsuru_app/tsuru_" + app.Name
	element["to"] = "tsuru_pool/tsuru_" + app.Pool
	return props
}

func (op *poolOperation) toEdge() *globomapPayload {
	return nil
}

func eventStatus(e event) string {
	parts := strings.Split(e.Kind.Name, ".")
	return strings.ToUpper(parts[1])
}
