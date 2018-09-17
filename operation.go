// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tsuru/globomap-integration/globomap"
	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
)

type operation interface {
	toPayload() *globomap.Payload
}

type nodeOperation struct {
	baseOperation
	nodeAddr string
}

type appOperation struct {
	baseOperation
	appName   string
	cachedApp *app
}

type appPoolOperation struct {
	baseOperation
	appName   string
	cachedApp *app
}

type poolOperation struct {
	baseOperation
	poolName string
}

type serviceOperation struct {
	baseOperation
	service tsuru.Service
}

type serviceInstanceOperation struct {
	baseOperation
	instance tsuru.ServiceInstance
}

type serviceServiceInstanceOperation struct {
	baseOperation
	instance tsuru.ServiceInstance
}

type appServiceInstanceOperation struct {
	baseOperation
	appName      string
	instanceName string
	serviceName  string
}

type baseOperation struct {
	action string
	time   time.Time
}

func (op *baseOperation) String() string {
	return fmt.Sprintf("[%s] %s", op.time.Format("2006-01-02 15:04:05"), op.action)
}

var (
	_ operation = &nodeOperation{}
	_ operation = &appPoolOperation{}
	_ operation = &appOperation{}
	_ operation = &poolOperation{}
	_ operation = &serviceOperation{}
	_ operation = &serviceInstanceOperation{}
	_ operation = &serviceServiceInstanceOperation{}
	_ operation = &appServiceInstanceOperation{}
)

func eventStatus(e event) string {
	parts := strings.Split(e.Kind.Name, ".")
	status := strings.ToUpper(parts[1])
	if status == "CREATE" {
		status = "UPDATE"
	}
	return status
}

func baseDocument(name, action, collection string, time time.Time, props map[string]interface{}) *globomap.Payload {
	doc := globomap.Payload{
		Action:     action,
		Collection: collection,
		Key:        "tsuru_" + name,
		Type:       globomap.PayloadTypeCollection,
	}

	if action == "DELETE" {
		return &doc
	}

	properties := map[string]interface{}{}
	propertiesMetadata := map[string]map[string]string{}
	for k, v := range props {
		properties[k] = v
		propertiesMetadata[k] = map[string]string{
			"description": k,
		}
	}

	doc.Element = map[string]interface{}{
		"id":                  name,
		"name":                name,
		"provider":            "tsuru",
		"timestamp":           time.Unix(),
		"properties":          properties,
		"properties_metadata": propertiesMetadata,
	}

	return &doc
}

func (op *appOperation) toPayload() *globomap.Payload {
	return baseDocument(op.appName, op.action, "tsuru_app", op.time, op.properties())
}

func (op *appOperation) String() string {
	return fmt.Sprintf("%s: app %s", op.baseOperation.String(), op.appName)
}

func (op *appOperation) app() (*app, error) {
	var err error
	if op.cachedApp == nil {
		op.cachedApp, err = env.tsuru.AppInfo(op.appName)
	}
	return op.cachedApp, err
}

func (op *appOperation) properties() map[string]interface{} {
	app, _ := op.app()
	if app == nil {
		return nil
	}

	props := map[string]interface{}{
		"description": app.Description,
		"tags":        app.Tags,
		"platform":    app.Platform,
		"addresses":   app.Addresses(),
		"router":      app.Router,
		"owner":       app.Owner,
		"team_owner":  app.TeamOwner,
		"teams":       app.Teams,
	}

	if app.Plan != nil {
		props["plan_name"] = app.Plan.Name
		props["plan_router"] = app.Plan.Router
		props["plan_memory"] = strconv.FormatInt(app.Plan.Memory, 10)
		props["plan_swap"] = strconv.FormatInt(app.Plan.Swap, 10)
		props["plan_cpushare"] = strconv.Itoa(int(app.Plan.Cpushare))
	}

	return props
}

func (op *appPoolOperation) app() (*app, error) {
	var err error
	if op.cachedApp == nil {
		op.cachedApp, err = env.tsuru.AppInfo(op.appName)
	}
	return op.cachedApp, err
}

func (op *appPoolOperation) toPayload() *globomap.Payload {
	id := fmt.Sprintf("%s-pool", op.appName)
	props := globomap.Payload{
		Action:     op.action,
		Collection: "tsuru_pool_app",
		Type:       globomap.PayloadTypeEdge,
		Key:        "tsuru_" + id,
	}

	if props.Action == "DELETE" {
		return &props
	}

	app, err := op.app()
	if err != nil {
		return nil
	}
	props.Element = map[string]interface{}{
		"id":        id,
		"name":      id,
		"provider":  "tsuru",
		"timestamp": op.time.Unix(),
		"from":      "tsuru_app/tsuru_" + app.Name,
		"to":        "tsuru_pool/tsuru_" + app.Pool,
	}
	return &props
}

func (op *poolOperation) toPayload() *globomap.Payload {
	return baseDocument(op.poolName, op.action, "tsuru_pool", op.time, op.properties())
}

func (op *poolOperation) String() string {
	return fmt.Sprintf("%s: pool %s", op.baseOperation.String(), op.poolName)
}

func (op *poolOperation) pool() *pool {
	for _, p := range env.pools {
		if p.Name == op.poolName {
			return &p
		}
	}
	return nil
}

func (op *poolOperation) properties() map[string]interface{} {
	pool := op.pool()
	if pool == nil {
		return nil
	}

	return map[string]interface{}{
		"provisioner": pool.Provisioner,
		"default":     strconv.FormatBool(pool.Default_),
		"public":      strconv.FormatBool(pool.Public),
		"teams":       pool.Teams,
	}
}

func (op *nodeOperation) toPayload() *globomap.Payload {
	return op.buildPayload(nil)
}

func (op *nodeOperation) buildPayload(queryResult *globomap.QueryResult) *globomap.Payload {
	ip := op.nodeIP()
	edge := globomap.Payload{
		Action:     op.action,
		Collection: "tsuru_pool_comp_unit",
		Type:       globomap.PayloadTypeEdge,
		Key:        "tsuru_" + strings.Replace(ip, ".", "_", -1),
	}

	if edge.Action == "DELETE" {
		return &edge
	}

	node, err := op.node()
	if err != nil || node == nil {
		return nil
	}

	if queryResult == nil {
		queryResult, err = env.globomap.Query(globomap.QueryFields{
			Collection: "comp_unit",
			Name:       node.Name(),
			IP:         node.IP(),
		})
		if err != nil || queryResult == nil {
			if env.config.repeat != nil {
				go op.retry()
			}
			if env.config.verbose {
				fmt.Printf("node %s (IP %s) not found in globomap API\n", node.Name(), node.IP())
			}
			return nil
		}
	}

	edge.Element = map[string]interface{}{
		"id":        ip,
		"name":      node.Name(),
		"provider":  "tsuru",
		"timestamp": op.time.Unix(),
		"from":      "tsuru_pool/tsuru_" + node.Pool,
		"to":        queryResult.Id,
		"properties": map[string]interface{}{
			"address": node.Addr(),
		},
		"properties_metadata": map[string]map[string]string{
			"address": {"description": "address"},
		},
	}

	return &edge
}

func (op *nodeOperation) String() string {
	return fmt.Sprintf("%s: node %s", op.baseOperation.String(), op.nodeAddr)
}

func (op *nodeOperation) node() (*node, error) {
	if len(env.nodes) == 0 {
		nodes, err := env.tsuru.NodeList()
		if err != nil {
			return nil, err
		}
		env.nodes = nodes
	}
	ip := op.nodeIP()
	for _, node := range env.nodes {
		if extractIPFromAddr(node.Address) == ip {
			return &node, nil
		}
	}
	if env.config.verbose {
		fmt.Printf("Node not found in tsuru API: %s\n", op.nodeAddr)
	}

	return nil, nil
}

func (op *nodeOperation) nodeIP() string {
	return extractIPFromAddr(op.nodeAddr)
}

func (op *nodeOperation) retry() {
	node, err := op.node()
	if err != nil || node == nil {
		return
	}
	f := globomap.QueryFields{
		Collection: "comp_unit",
		Name:       node.Name(),
		IP:         node.IP(),
	}

	for i := 1; i <= env.config.maxRetries; i++ {
		retrySleepTime := env.config.retrySleepTime * time.Duration(i)
		if env.config.verbose {
			fmt.Printf("(%d/%d) retrying globomap query in %s\n", i, env.config.maxRetries, retrySleepTime)
		}
		time.Sleep(retrySleepTime)

		queryResult, err := env.globomap.Query(f)
		if queryResult == nil || err != nil {
			if env.config.verbose {
				fmt.Printf("node %s (IP %s) not found in globomap API\n", node.Name(), node.IP())
			}
			continue
		}

		payload := op.buildPayload(queryResult)
		if payload == nil {
			return
		}
		err = env.globomap.Post([]globomap.Payload{*payload})
		if err != nil && env.config.verbose {
			fmt.Println(err)
		}
		return
	}

	fmt.Printf("max retries reached for fetching node %s (IP %s) from globomap API, giving up\n", node.Name(), node.IP())
}

func (op *serviceOperation) toPayload() *globomap.Payload {
	planMap := make(map[string]struct{})
	for _, p := range op.service.Plans {
		planMap[p] = struct{}{}
	}
	plans := make([]string, len(planMap))
	var i int
	for p := range planMap {
		plans[i] = p
		i++
	}
	return baseDocument(op.service.Service, op.action, "tsuru_service", op.time, map[string]interface{}{
		"plans": plans,
	})
}

func (op *serviceOperation) String() string {
	return fmt.Sprintf("%s: service %s", op.baseOperation.String(), op.service.Service)
}

func (op *serviceInstanceOperation) toPayload() *globomap.Payload {
	return baseDocument(op.instance.ServiceName+"_"+op.instance.Name, op.action, "tsuru_service_instance", time.Now(), map[string]interface{}{
		"plan":        op.instance.PlanName,
		"description": op.instance.Description,
		"tags":        op.instance.Tags,
		"team_owner":  op.instance.TeamOwner,
		"teams":       op.instance.Teams,
	})
}

func (op *serviceInstanceOperation) String() string {
	return fmt.Sprintf("%s: service instance %v service %v", op.baseOperation.String(), op.instance.Name, op.instance.ServiceName)
}

func (op *serviceServiceInstanceOperation) toPayload() *globomap.Payload {
	id := op.instance.ServiceName + "_" + op.instance.Name
	return &globomap.Payload{
		Action:     op.action,
		Collection: "tsuru_service_service_instance",
		Type:       globomap.PayloadTypeEdge,
		Key:        "tsuru_" + id,
		Element: map[string]interface{}{
			"id":        id,
			"name":      id,
			"provider":  "tsuru",
			"timestamp": op.time.Unix(),
			"from":      "tsuru_service/tsuru_" + op.instance.ServiceName,
			"to":        "tsuru_service_instance/tsuru_" + id,
		},
	}
}

func (op *appServiceInstanceOperation) toPayload() *globomap.Payload {
	id := op.appName + "_" + op.instanceName
	return &globomap.Payload{
		Action:     op.action,
		Collection: "tsuru_app_service_instance",
		Type:       globomap.PayloadTypeEdge,
		Key:        "tsuru_" + id,
		Element: map[string]interface{}{
			"id":        id,
			"name":      id,
			"provider":  "tsuru",
			"timestamp": op.time.Unix(),
			"from":      "tsuru_app/tsuru_" + op.appName,
			"to":        "tsuru_service_instance/tsuru_" + op.serviceName + "_" + op.instanceName,
		},
	}
}

func extractIPFromAddr(addr string) string {
	re := regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)`)
	matches := re.FindAllStringSubmatch(addr, -1)
	if len(matches) == 1 {
		return matches[0][1]
	}
	return ""
}
