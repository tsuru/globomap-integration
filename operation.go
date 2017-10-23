// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"
	"time"
)

type operation struct {
	name       string
	collection string
	docType    string
	events     []event
	app        *app
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

func eventStatus(e event) string {
	parts := strings.Split(e.Kind.Name, ".")
	return strings.ToUpper(parts[1])
}

func (op *operation) toDocument() globomapPayload {
	props := globomapPayload{
		"action":     op.action(),
		"type":       op.docType,
		"collection": op.collection,
		"element": map[string]interface{}{
			"id":        op.name,
			"name":      op.name,
			"provider":  "tsuru",
			"timestamp": op.Time().Unix(),
		},
	}

	properties := make(map[string]interface{})
	propertiesMetadata := make(map[string]map[string]string)
	/*for k, v := range op.properties {
		properties[k] = v.value
		propertiesMetadata[k] = map[string]string{
			"description": k,
		}
	}*/

	element, _ := props["element"].(map[string]interface{})
	element["properties"] = properties
	element["properties_metadata"] = propertiesMetadata

	return props
}

func (op *operation) toEdge() globomapPayload {
	props := op.toDocument()
	from := op.app.Name
	to := op.app.Pool
	element, _ := props["element"].(map[string]interface{})
	element["id"] = fmt.Sprintf("%s-%s", from, to)
	element["name"] = fmt.Sprintf("%s-%s", from, to)
	element["from"] = from
	element["to"] = to
	return props
}
