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
	"net/url"
)

type globomapClient struct {
	LoaderHostname string
	ApiHostname    string
}

type globomapPayload map[string]interface{}

type globomapQueryResult struct {
	Id   string `json:"_id"`
	Name string
}

type globomapResponse struct {
	Message string `json:"message"`
}

func (g *globomapClient) Post(payload []globomapPayload) error {
	path := "/v1/updates"
	body := g.body(payload)
	if body == nil {
		return errors.New("No events to post")
	}
	resp, err := g.doPost(path, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}

	if env.config.dry {
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

func (g *globomapClient) QueryByName(collection, name string) ([]globomapQueryResult, error) {
	query := fmt.Sprintf(`[[{"field":"name","value":"%s","operator":"=="}]]`, name)
	path := fmt.Sprintf("/v1/collections/%s/?query=%s", collection, url.PathEscape(query))
	resp, err := http.Get(g.ApiHostname + path)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)
	var data struct {
		Documents []globomapQueryResult
	}
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return data.Documents, nil
}

func (g *globomapClient) doPost(path string, body io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, g.LoaderHostname+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	if env.config.dry {
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

func (g *globomapClient) body(data []globomapPayload) io.Reader {
	if len(data) == 0 {
		return nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return bytes.NewReader(b)
}
