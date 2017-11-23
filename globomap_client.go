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
	"math"
	"net/http"
	"net/url"
	"time"

	tsuruErrors "github.com/tsuru/tsuru/errors"
)

type globomapClient struct {
	LoaderHostname string
	ApiHostname    string
}

type globomapPayload map[string]interface{}

type globomapQueryFields struct {
	collection string
	name       string
	ip         string
}

type globomapQueryResult struct {
	Id         string `json:"_id"`
	Name       string
	Properties globomapProperties
}

type globomapProperties struct {
	IPs []string
}

type globomapResponse struct {
	JobID   string `json:"jobid"`
	Message string `json:"message"`
}

func (g *globomapClient) Post(payload []globomapPayload) error {
	maxPayloadItems := 100
	if len(payload) <= maxPayloadItems {
		return g.post(payload)
	}

	chunks := int(math.Ceil(float64(len(payload)) / float64(maxPayloadItems)))
	errs := tsuruErrors.NewMultiError()
	for i := 0; i < chunks; i++ {
		start := i * maxPayloadItems
		end := start + maxPayloadItems
		if end > len(payload) {
			end = len(payload)
		}

		if env.config.verbose {
			fmt.Printf("Posting chunk %d/%d\n", i+1, chunks)
		}
		err := g.post(payload[start:end])
		if err != nil {
			errs.Add(err)
		}
		time.Sleep(env.config.sleepTimeBetweenChunks)
	}

	if errs.Len() > 0 {
		return errs
	}
	return nil
}

func (g *globomapClient) post(payload []globomapPayload) error {
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
	fmt.Println(&data)
	return nil
}

func (g *globomapClient) Query(f globomapQueryFields) (*globomapQueryResult, error) {
	results, err := g.queryByName(f.collection, f.name)
	if err != nil {
		return nil, err
	}
	if len(results) == 1 {
		return &results[0], nil
	}
	for _, result := range results {
		for _, resultIP := range result.Properties.IPs {
			if resultIP == f.ip {
				return &result, nil
			}
		}
	}
	return nil, nil
}

func (g *globomapClient) queryByName(collection, name string) ([]globomapQueryResult, error) {
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
		fmt.Println(string(data))
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

func (r *globomapResponse) String() string {
	return fmt.Sprintf("[%s] %s", r.JobID, r.Message)
}
