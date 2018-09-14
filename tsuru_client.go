// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/tsuru/go-tsuruclient/pkg/tsuru"

	"gopkg.in/mgo.v2/bson"
)

const TIME_FORMAT = "2006-01-02T15:04:05-07:00"

type tsuruClient struct {
	Hostname string
	Token    string
}

type app tsuru.App

type pool tsuru.Pool

type node tsuru.Node

type event struct {
	Target struct {
		Type  string
		Value string
	}
	Kind struct {
		Name string
	}
	EndTime         time.Time
	Error           string
	EndCustomData   bson.Raw
	StartCustomData bson.Raw
}

type eventFilter struct {
	Kindnames  []string
	TargetType string
	Since      *time.Time
	Until      *time.Time
}

func (a *app) Addresses() []string {
	return append(a.Cname, a.Ip)
}

func (e *event) Failed() bool {
	return e.Error != ""
}

func (e *event) EndData(value interface{}) error {
	if e.EndCustomData.Kind == 0 {
		return nil
	}
	return e.EndCustomData.Unmarshal(value)
}

func (n *node) Name() string {
	return n.Iaasid
}

func (n *node) Addr() string {
	return n.Address
}

func (n *node) IP() string {
	return extractIPFromAddr(n.Address)
}

func (t *tsuruClient) EventList(f eventFilter) ([]event, error) {
	path := "/events"
	resp, err := t.doRequest(path + f.format())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var events []event
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&events)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return events, nil
}

func (t *tsuruClient) AppList() ([]tsuru.MiniApp, error) {
	apps, _, err := t.apiClient().AppApi.AppList(context.Background(), make(map[string]interface{}))
	if err != nil {
		return nil, err
	}
	return apps, nil
}

func (t *tsuruClient) AppInfo(name string) (*app, error) {
	a, _, err := t.apiClient().AppApi.AppGet(context.Background(), name)
	if err != nil {
		return nil, err
	}
	iApp := app(a)
	return &iApp, nil
}

func (t *tsuruClient) PoolList() ([]pool, error) {
	poolList, _, err := t.apiClient().PoolApi.PoolList(context.Background())
	if err != nil {
		return nil, err
	}
	pools := make([]pool, len(poolList))
	for i := range poolList {
		pools[i] = pool(poolList[i])
	}
	return pools, nil
}

func (t *tsuruClient) NodeList() ([]node, error) {
	nodeList, _, err := t.apiClient().NodeApi.NodeList(context.Background())
	if err != nil {
		return nil, err
	}
	nodes := make([]node, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		nodes[i] = node(nodeList.Nodes[i])
	}
	return nodes, nil
}

func (t *tsuruClient) ServiceList() ([]tsuru.Service, error) {
	services, _, err := t.apiClient().ServiceApi.InstancesList(context.Background(), map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return services, nil
}

func (t *tsuruClient) doRequest(path string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, t.Hostname+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "b "+t.Token)
	return client.Do(req)
}

func (t *tsuruClient) apiClient() *tsuru.APIClient {
	cfg := tsuru.Configuration{
		BasePath: t.Hostname,
		DefaultHeader: map[string]string{
			"Authorization": "bearer " + t.Token,
		},
	}
	return tsuru.NewAPIClient(&cfg)
}

func (f *eventFilter) format() string {
	v := url.Values{}
	v.Set("running", "false")
	for _, k := range f.Kindnames {
		v.Add("kindname", k)
	}
	if f.TargetType != "" {
		v.Set("target.type", f.TargetType)
	}
	if f.Since != nil {
		v.Set("since", f.Since.Format(TIME_FORMAT))
	}
	if f.Until != nil {
		v.Set("until", f.Until.Format(TIME_FORMAT))
	}

	return "?" + v.Encode()
}
