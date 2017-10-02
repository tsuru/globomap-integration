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

type event struct {
	Target struct {
		Type  string
		Value string
	}
	Kind struct {
		Name string
	}
	EndTime time.Time
}

type eventFilter struct {
	Kindname string
	Since    *time.Time
	Until    *time.Time
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
