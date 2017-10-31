// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

const TIME_FORMAT = "2006-01-02T15:04:05MST"

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
	Pool     string
	Status   string
	Metadata struct {
		IaasID string `json:"iaas-id"`
	}
}

type event struct {
	Target struct {
		Type  string
		Value string
	}
	Kind struct {
		Name string
	}
	EndTime time.Time
	Error   string
}

type eventFilter struct {
	Kindname string
	Since    *time.Time
	Until    *time.Time
}

func (a *app) Addresses() []string {
	return append(a.Cname, a.Ip)
}

func (e *event) Failed() bool {
	return e.Error != ""
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
	if f.Kindname != "" {
		v.Set("kindname", f.Kindname)
	}
	if f.Since != nil {
		v.Set("since", f.Since.Format(TIME_FORMAT))
	}
	if f.Until != nil {
		v.Set("until", f.Until.Format(TIME_FORMAT))
	}

	return "?" + v.Encode()
}
