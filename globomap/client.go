// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package globomap

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

type PayloadType string

const (
	PayloadTypeCollection = PayloadType("collections")
	PayloadTypeEdge       = PayloadType("edges")
)

type Client struct {
	LoaderHostname string
	ApiHostname    string
	Username       string
	Password       string

	// ChunckInterval controls the interval between each chunk of updates
	// sent to the Globomap Loader.
	ChunkInterval time.Duration
	Verbose       bool
	Dry           bool

	token *token
}

type Payload struct {
	Collection string                 `json:"collection"`
	Action     string                 `json:"action"`
	Type       PayloadType            `json:"type"`
	Key        string                 `json:"key"`
	Element    map[string]interface{} `json:"element"`
}

type QueryFields struct {
	Collection string `json:"collection"`
	Name       string `json:"name"`
	IP         string `json:"ip"`
}

type QueryResult struct {
	Id         string `json:"_id"`
	Name       string
	Properties Properties
}

type Properties struct {
	IPs []string
}

type response struct {
	JobID   string `json:"jobid"`
	Message string `json:"message"`
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type token struct {
	Token string `json:"token"`
}

func (g *Client) Post(payload []Payload) error {
	if err := g.auth(g.LoaderHostname); err != nil {
		return fmt.Errorf("failed to authenticate with globomap loader: %v", err)
	}
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

		if g.Verbose {
			fmt.Printf("Posting chunk %d/%d\n", i+1, chunks)
		}
		err := g.post(payload[start:end])
		if err != nil {
			errs.Add(err)
		}
		time.Sleep(g.ChunkInterval)
	}

	if errs.Len() > 0 {
		return errs
	}
	return nil
}

func (g *Client) Query(f QueryFields) (*QueryResult, error) {
	if err := g.auth(g.ApiHostname); err != nil {
		return nil, fmt.Errorf("failed to authenticate with globomap API: %v", err)
	}
	results, err := g.queryByName(f.Collection, f.Name)
	if err != nil {
		return nil, err
	}
	if len(results) == 1 {
		return &results[0], nil
	}
	for _, result := range results {
		for _, resultIP := range result.Properties.IPs {
			if resultIP == f.IP {
				return &result, nil
			}
		}
	}
	return nil, nil
}

func (g *Client) auth(addr string) error {
	if g.Username == "" && g.Password == "" {
		return nil
	}
	g.token = new(token)
	req := authRequest{Username: g.Username, Password: g.Password}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(req); err != nil {
		return err
	}
	resp, err := g.doPost(addr, "/v2/auth/", buf)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response code for auth: %v", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(g.token)
}

func (g *Client) post(payload []Payload) error {
	path := "/v1/updates"
	if g.Username != "" || g.Password != "" {
		path = "/v2/updates/"
	}
	body := g.body(payload)
	if body == nil {
		return errors.New("No events to post")
	}
	resp, err := g.doPost(g.LoaderHostname, path, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted {
		return errors.New(resp.Status)
	}

	if g.Dry {
		return nil
	}

	decoder := json.NewDecoder(resp.Body)
	var data response
	err = decoder.Decode(&data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println(&data)
	return nil
}

func (g *Client) queryByName(collection, name string) ([]QueryResult, error) {
	path := "/v1/collections"
	if g.Username != "" || g.Password != "" {
		path = "/v2/collections"
	}
	query := fmt.Sprintf(`[[{"field":"name","value":"%s","operator":"=="}]]`, name)
	path = fmt.Sprintf("%s/%s/?query=%s", path, collection, url.PathEscape(query))
	resp, err := http.Get(g.ApiHostname + path)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)
	var data struct {
		Documents []QueryResult
	}
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return data.Documents, nil
}

func (g *Client) doPost(addr, path string, body io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, addr+path, body)
	if err != nil {
		return nil, err
	}
	if g.token != nil {
		req.Header.Add("Authorization", fmt.Sprintf("Token token=%s", g.token.Token))
	}
	req.Header.Add("x-driver-name", "tsuru")
	req.Header.Add("Content-Type", "application/json")
	if g.Dry {
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

func (g *Client) body(data []Payload) io.Reader {
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

func (r *response) String() string {
	return fmt.Sprintf("[%s] %s", r.JobID, r.Message)
}
