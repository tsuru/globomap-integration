// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"io"
	"net/http"
)

const TIME_FORMAT = "2006-01-02T15:04:05MST"

type globomapClient struct {
	Hostname string
}

type globomapDocument struct {
	id         string
	name       string
	properties map[string]globomapProperty
	timestamp  int
}

type globomapProperty struct {
	name        string
	description string
	value       interface{}
}

func (g *globomapClient) Create() error {
	path := "/v1/updates"
	resp, err := g.doRequest(path)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return nil
}

func (g *globomapClient) doRequest(path string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, g.Hostname+path, g.body())
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (g *globomapClient) body() io.Reader {
	return nil
}

func (d *globomapDocument) export() map[string]interface{} {
	return map[string]interface{}{
		"action":     "CREATE",
		"collection": "tsuru_app",
		"element": map[string]interface{}{
			"id":   "cartola-api-prod",
			"name": "cartola-api-prod",
			"properties": map[string]interface{}{
				"platform":    "static",
				"description": "Cartola API (PROD)",
			},
			"properties_metadata": map[string]interface{}{
				"platform": map[string]string{
					"description": "App Platform",
				},
				"description": map[string]string{
					"description": "Description",
				},
			},
			"provider":  "tsuru",
			"timestamp": 1506535249,
		},
		"type": "collection",
	}
}
