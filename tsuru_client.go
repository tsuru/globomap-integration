// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"gopkg.in/mgo.v2/bson"
)

const TIME_FORMAT = "2006-01-02T15:04:05-07:00"

type tsuruClient struct {
	Hostname string
	Token    string
}

type app struct {
	Name        string
	Pool        string
	Description string
	Tags        []string
	Platform    string
	Router      string
	Teams       []string
	Ip          string
	Cname       []string
	Owner       string
	TeamOwner   string
	Plan        appPlan
}

type appPlan struct {
	Cpushare int
	Memory   int
	Name     string
	Router   string
	Swap     int
}

type pool struct {
	Name        string
	Provisioner string
	Default     bool
	Public      bool
	Teams       []string
}

type node struct {
	Id       string
	Address  string
	Port     int
	Protocol string
	Pool     string
	Status   string
	IaaSID   string
}

type event struct {
	Target struct {
		Type  string
		Value string
	}
	Kind struct {
		Name string
	}
	EndTime       time.Time
	Error         string
	EndCustomData bson.Raw
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
	return n.IaaSID
}

func (n *node) Addr() string {
	addr := n.Address
	if n.Protocol != "" {
		addr = fmt.Sprintf("%s://%s", n.Protocol, addr)
	}
	if n.Port != 0 {
		addr = fmt.Sprintf("%s:%d", addr, n.Port)
	}
	return addr
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

func (t *tsuruClient) AppList() ([]app, error) {
	path := "/apps"
	resp, err := t.doRequest(path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var apps []app
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&apps)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return apps, nil
}

func (t *tsuruClient) AppInfo(name string) (*app, error) {
	path := "/apps/" + name
	resp, err := t.doRequest(path)
	if err != nil {
		return nil, err
	}

	var a app
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&a)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return &a, nil
}

func (t *tsuruClient) PoolList() ([]pool, error) {
	path := "/pools"
	resp, err := t.doRequest(path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var pools []pool
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&pools)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return pools, nil
}

func (t *tsuruClient) NodeList() ([]node, error) {
	path := "/node"
	resp, err := t.doRequest(path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var data struct {
		Nodes []node
	}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return data.Nodes, nil
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
