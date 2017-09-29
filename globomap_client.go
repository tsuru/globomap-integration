// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type globomapClient struct {
	Hostname string
}

type globomapDocument struct {
	id         string
	name       string
	properties map[string]globomapProperty
	timestamp  int64
}

type globomapProperty struct {
	name        string
	description string
	value       interface{}
}

func (g *globomapClient) Create(ops []operation) error {
	path := "/v1/updates"
	resp, err := g.doRequest(path, g.body(ops))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return nil
}

func (g *globomapClient) doRequest(path string, body io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, g.Hostname+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	return client.Do(req)
}

func (g *globomapClient) body(ops []operation) io.Reader {
	data := make([]map[string]interface{}, len(ops))
	for i, op := range ops {
		doc := newDocument(op)
		data[i] = doc.export()
	}
	b, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return bytes.NewReader(b)
}

func newDocument(op operation) globomapDocument {
	return globomapDocument{
		name:      op.appName,
		timestamp: op.events[len(op.events)-1].EndTime.Unix(),
	}
}

func (d *globomapDocument) export() map[string]interface{} {
	props := map[string]interface{}{
		"action":     "CREATE",
		"collection": "tsuru_app",
		"element": map[string]interface{}{
			"id":        d.name,
			"name":      d.name,
			"provider":  "tsuru",
			"timestamp": d.timestamp,
		},
		"type": "collection",
	}

	properties := make(map[string]interface{})
	propertiesMetadata := make(map[string]map[string]string)
	for k, v := range d.properties {
		properties[k] = v.value
		propertiesMetadata[k] = map[string]string{
			"description": k,
		}
	}

	element, _ := props["element"].(map[string]interface{})
	element["properties"] = properties
	element["properties_metadata"] = propertiesMetadata

	return props
}
