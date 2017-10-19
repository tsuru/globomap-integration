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
	"io/ioutil"
	"net/http"
)

type globomapClient struct {
	Hostname string
}

type globomapDocument struct {
	collection string
	id         string
	name       string
	properties map[string]globomapProperty
	timestamp  int64
	docType    string
}

type globomapEdge struct {
	globomapDocument
	from string
	to   string
}

type globomapPayload map[string]interface{}

type globomapProperty struct {
	name        string
	description string
	value       interface{}
}

type globomapResponse struct {
	Message string `json:"message"`
}

func (g *globomapClient) Post(ops []operation) error {
	path := "/v1/updates"
	resp, err := g.doRequest(path, g.body(ops))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}

	if dry {
		return nil
	}

	decoder := json.NewDecoder(resp.Body)
	var data globomapResponse
	err = decoder.Decode(&data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println(data.Message)
	return nil
}

func (g *globomapClient) doRequest(path string, body io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, g.Hostname+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	if dry {
		data, err := ioutil.ReadAll(body)
		if err != nil {
			return nil, err
		}
		fmt.Printf("%s\n", data)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "OK",
		}
		return resp, nil
	}
	return client.Do(req)
}

func (g *globomapClient) body(ops []operation) io.Reader {
	data := make([]globomapPayload, len(ops))
	for i, op := range ops {
		if op.docType == "collections" {
			doc := newDocument(op)
			data[i] = doc.export()
		} else {
			edge := newEdge(op)
			data[i] = edge.export()
		}
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
		name:       op.name,
		collection: op.collection,
		docType:    op.docType,
		timestamp:  op.Time().Unix(),
	}
}

func newEdge(op operation) globomapEdge {
	edge := globomapEdge{
		globomapDocument: newDocument(op),
		from:             op.app.Name,
		to:               op.app.Pool,
	}

	return edge
}

func (d *globomapDocument) export() globomapPayload {
	props := map[string]interface{}{
		"action":     "CREATE",
		"type":       d.docType,
		"collection": d.collection,
		"element": map[string]interface{}{
			"id":        d.name,
			"name":      d.name,
			"provider":  "tsuru",
			"timestamp": d.timestamp,
		},
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

func (e *globomapEdge) export() globomapPayload {
	props := e.globomapDocument.export()
	element, _ := props["element"].(map[string]interface{})
	element["id"] = fmt.Sprintf("%s-%s", e.from, e.to)
	element["name"] = fmt.Sprintf("%s-%s", e.from, e.to)
	element["from"] = e.from
	element["to"] = e.to
	return props
}
