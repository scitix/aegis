package uticket

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

const (
	listNodePath       string = "/ucp/api/v1/node/listNodes"
	cordonNodePath     string = "/ucp/api/v1/node/cordonNode"
	uncordonNodePath   string = "/ucp/api/v1/node/uncordonNode"
	drainNodePath      string = "/ucp/api/v1/node/drainNode"
	addLabelPath       string = "/ucp/api/v1/node/addLabel"
	deleteLabelPath    string = "/ucp/api/v1/node/delLabel"
	createTicketPath   string = "/ucp/api/v1/support/createTicket"
	getTicketPath      string = "/ucp/api/v1/support/getTicket"
	patchTicketPath    string = "/ucp/api/v1/support/patchTicket"
	acceptTicketPath   string = "/ucp/api/v1/support/acceptTicket"
	dispatchTicketPath string = "/ucp/api/v1/support/dispatchTicket"
	resolveTicketPath  string = "/ucp/api/v1/support/resolveTicket"
	getMachinePath     string = "/ucp/api/v1/machine/listMachines"
	listTicketsPath    string = "/ucp/api/v1/support/listOwnTickets"
	deleteTicketPath   string = "/ucp/api/v1/support/delTicket"
)

type Client struct {
	endpoint string
}

func CreateClient(endpoint string) (*Client, error) {
	_, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("Invalid url endpoint %s: %v", endpoint, err)
	}

	return &Client{
		endpoint: strings.TrimSuffix(endpoint, "/"),
	}, nil
}

func getHeader() map[string]string {
	appID := os.Getenv("AppID")
	ranstr := os.Getenv("Ranstr")

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	strA := appID + ranstr + timestamp
	hash := sha256.Sum256([]byte(strA))

	return map[string]string{
		"Signature": hex.EncodeToString(hash[:]),
		"AppID":     appID,
		"Randstr":   ranstr,
		"Timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
}

func (u *Client) NodeExists(ctx context.Context, clusterName, node string) bool {
	defer func() {
		if err := recover(); err != nil {
			klog.Warningf("recover from panic: %v", err)
		}
	}()

	params := map[string]string{
		"clusterName": clusterName,
		"nodeNames":   node,
	}
	headers := getHeader()
	address := u.endpoint + listNodePath

	object, err := get(ctx, address, params, headers)
	klog.Infof("request address: %s, params: %v, response object: %+v", address, params, object)

	if err != nil || object == nil {
		klog.Infof("err: %s, object: %+v", err, object)
		return false
	}

	return true
}

func (u *Client) CordonNode(ctx context.Context, clusterName, node, reason, remark string) error {
	data := map[string]interface{}{
		"clusterName": clusterName,
		"names":       []string{node},
		"reason":      reason,
		"remark":      remark,
	}
	headers := getHeader()
	address := u.endpoint + cordonNodePath

	return post(ctx, address, data, headers)
}

func (u *Client) UncordonNode(ctx context.Context, clusterName, node, remark string) error {
	data := map[string]interface{}{
		"clusterName": clusterName,
		"names":       []string{node},
		"remark":      remark,
	}
	headers := getHeader()
	address := u.endpoint + uncordonNodePath

	return post(ctx, address, data, headers)
}

func (u *Client) DrainNode(ctx context.Context, clusterName, node, reason, remark string) error {
	data := map[string]interface{}{
		"clusterName": clusterName,
		"names":       []string{node},
		"reason":      reason,
		"remark":      remark,
	}
	headers := getHeader()
	address := u.endpoint + drainNodePath

	return post(ctx, address, data, headers)
}

func (u *Client) AddNodeLabel(ctx context.Context, cluster, node, label, reason string) error {
	data := map[string]interface{}{
		"clusterName": cluster,
		"names":       []string{node},
		"label":       label,
		"reason":      reason,
	}

	headers := getHeader()
	address := u.endpoint + addLabelPath

	return post(ctx, address, data, headers)
}

func (u *Client) DeleteNodeLabel(ctx context.Context, cluster, node, label, reason string) error {
	data := map[string]interface{}{
		"clusterName": cluster,
		"names":       []string{node},
		"label":       label,
		"reason":      reason,
	}

	headers := getHeader()
	address := u.endpoint + deleteLabelPath

	return post(ctx, address, data, headers)
}

func (u *Client) CreateTicket(ctx context.Context, clusterName, node, title, description, hardwareType string, rangerType string) error {
	data := map[string]interface{}{
		"cluster":         clusterName,
		"creator":         "aegis",
		"title":           title,
		"priority":        "high",
		"node":            node,
		"isHardwareIssue": true,
		"model":           "hardware",
		"hardwareType":    hardwareType,
		"ranger":          rangerType,
		"startTime":       time.Now().Format("2006-01-02T15:04:05Z07:00"),
		"description":     description,
		"isFromCustomer":  false,
	}

	headers := getHeader()
	address := u.endpoint + createTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) DeleteTicket(ctx context.Context, ticketId string) error {
	data := map[string]interface{}{
		"ticketID": ticketId,
	}

	headers := getHeader()
	address := u.endpoint + deleteTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) CreateComponentTicket(ctx context.Context, clusterName, node, title, description, model, component string) error {
	data := map[string]interface{}{
		"cluster":         clusterName,
		"creator":         "aegis",
		"title":           title,
		"priority":        "high",
		"node":            node,
		"isHardwareIssue": false,
		"model":           model,
		"component":       component,
		"startTime":       time.Now().Format("2006-01-02T15:04:05Z07:00"),
		"description":     description,
		"isFromCustomer":  false,
	}

	headers := getHeader()
	address := u.endpoint + createTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) CreateExtraUniqTicket(ctx context.Context, clusterName, uniqueName, title, description, model, supervisor string) error {
	data := map[string]interface{}{
		"cluster":         clusterName,
		"creator":         "aegis",
		"title":           title,
		"priority":        "high",
		"extraUniqName":   uniqueName,
		"isHardwareIssue": true,
		"supervisor":      supervisor,
		"model":           model,
		"startTime":       time.Now().Format("2006-01-02T15:04:05Z07:00"),
		"description":     description,
		"isFromCustomer":  false,
	}

	headers := getHeader()
	address := u.endpoint + createTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) PatchTicket(ctx context.Context, ticketId, key, value string) error {
	data := map[string]interface{}{
		"ticketID": ticketId,
		"key":      key,
		"value":    value,
	}

	headers := getHeader()
	address := u.endpoint + patchTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) GetTicket(ctx context.Context, ticketId string) (*TicketInfo, error) {
	data := map[string]string{
		"ticketID": ticketId,
	}

	headers := getHeader()
	address := u.endpoint + getTicketPath

	result, err := get(ctx, address, data, headers)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	res := result.(map[string]interface{})

	info := &TicketInfo{
		TicketId:        res["ticketID"].(string),
		Status:          TicketStatus(res["status"].(string)),
		Cluster:         res["cluster"].(string),
		Node:            res["node"].(string),
		IsHardwareIssue: res["isHardwareIssue"].(bool),
		Creator:         res["creator"].(string),
		Description:     res["description"].(string),
		Supervisor:      res["supervisor"].(string),
		Title:           res["title"].(string),
	}

	return info, nil
}

func (u *Client) AcceptTicket(ctx context.Context, ticketId string) error {
	data := map[string]interface{}{
		"ticketID": ticketId,
		"status":   "resolving",
	}

	headers := getHeader()
	address := u.endpoint + acceptTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) DispatchTicket(ctx context.Context, ticketId string, user string) error {
	data := map[string]interface{}{
		"ticketID":   ticketId,
		"supervisor": user,
		"status":     "assigned",
	}

	headers := getHeader()
	address := u.endpoint + dispatchTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) ResolveTicket(ctx context.Context, ticketId, clusterName, nodeName, answer, operation string, isHardwareIssue bool) error {
	data := map[string]interface{}{
		"ticketID":        ticketId,
		"answer":          answer,
		"operation":       operation,
		"finishTime":      time.Now().Format("2006-01-02T15:04:05Z07:00"),
		"isHardwareIssue": isHardwareIssue,
		"node":            nodeName,
		"cluster":         clusterName,
		"team":            "K8s",
	}

	headers := getHeader()
	address := u.endpoint + resolveTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) CloseTicket(ctx context.Context, ticketId string) error {
	data := map[string]interface{}{
		"ticketID": ticketId,
		"key":      "status",
		"value":    "closed",
	}

	headers := getHeader()
	address := u.endpoint + patchTicketPath

	return post(ctx, address, data, headers)
}

func (u *Client) GetMachineInfo(ctx context.Context, clusterName, node string) (*MachineInfo, error) {
	data := map[string]string{
		"clusterName": clusterName,
		"nodeName":    node,
	}

	headers := getHeader()
	address := u.endpoint + getMachinePath

	result, err := get(ctx, address, data, headers)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	res := result.(map[string]interface{})

	info := &MachineInfo{
		ClusterName: res["clusterName"].(string),
		NodeName:    res["nodeName"].(string),
		Status:      MachineStatus(res["status"].(string)),
		TicketId:    res["ticketID"].(string),
	}

	return info, nil
}

func (u *Client) GetNodeFirstUnResovledTicket(ctx context.Context, clusterName, node string) (*TicketInfo, error) {
	machine, err := u.GetMachineInfo(ctx, clusterName, node)
	if err != nil {
		return nil, err
	}

	if machine != nil && machine.TicketId != "" {
		return u.GetTicket(ctx, machine.TicketId)
	}

	data := map[string]string{
		"cluster":  clusterName,
		"node":     node,
		"page":     "1",
		"pageSize": "15",
	}

	headers := getHeader()
	address := u.endpoint + listTicketsPath

	result, err := get(ctx, address, data, headers)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	res := result.(map[string]interface{})

	info := &TicketInfo{
		TicketId:        res["ticketID"].(string),
		Status:          TicketStatus(res["status"].(string)),
		Cluster:         res["cluster"].(string),
		Node:            res["node"].(string),
		IsHardwareIssue: res["isHardwareIssue"].(bool),
		Creator:         res["creator"].(string),
		Description:     res["description"].(string),
		Supervisor:      res["supervisor"].(string),
		Title:           res["title"].(string),
	}

	if info.Status == "resolved" || info.Status == "closed" {
		return nil, nil
	}

	return info, nil
}

func (u *Client) ListNodeTickets(ctx context.Context, clusterName, node string, size int) ([]*TicketInfo, error) {
	data := map[string]string{
		"cluster":  clusterName,
		"node":     node,
		"page":     "1",
		"pageSize": strconv.Itoa(size),
	}

	headers := getHeader()
	address := u.endpoint + listTicketsPath
	results, err := getAll(ctx, address, data, headers)
	if err != nil {
		return nil, err
	}

	if results == nil {
		return nil, nil
	}

	res := make([]*TicketInfo, 0)
	for _, t := range results {
		r := t.(map[string]interface{})
		info := &TicketInfo{
			TicketId:        r["ticketID"].(string),
			Status:          TicketStatus(r["status"].(string)),
			Cluster:         r["cluster"].(string),
			Node:            r["node"].(string),
			IsHardwareIssue: r["isHardwareIssue"].(bool),
			Creator:         r["creator"].(string),
			Description:     r["description"].(string),
			Supervisor:      r["supervisor"].(string),
			Title:           r["title"].(string),
		}

		res = append(res, info)
	}

	return res, nil
}

func post(ctx context.Context, address string, data map[string]interface{}, headers map[string]string) error {
	body, err := json.Marshal(data)
	if err != nil {

		return fmt.Errorf("Marshal data %v failed: %v", data, err)
	}
	bodyReader := bytes.NewReader(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bodyReader)
	if err != nil {
		return fmt.Errorf("Create request failed: %v", err)
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Error making http request: %v", err)
	}

	resBody, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Post data %v, statuscode: %v, body: %v", data, res.StatusCode, string(resBody))
	}

	rMap := make(map[string]interface{})
	err = json.Unmarshal(resBody, &rMap)
	if err != nil {
		return err
	}

	if !rMap["status"].(bool) {
		return errors.New(rMap["error"].(string))
	}

	return nil
}

func get(ctx context.Context, address string, params map[string]string, headers map[string]string) (interface{}, error) {
	rs, err := getAll(ctx, address, params, headers)
	if err != nil {
		return nil, err
	}

	if len(rs) == 0 {
		return nil, nil
	}

	return rs[0], err
}

func getAll(ctx context.Context, address string, params map[string]string, headers map[string]string) ([]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("Create request failed: %v", err)
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	query := req.URL.Query()
	for key, value := range params {
		query.Add(key, value)
	}
	req.URL.RawQuery = query.Encode()

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error making http request: %v", err)
	}

	resBody, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Get statuscode: %v, body: %v", res.StatusCode, resBody)
	}

	rMap := make(map[string]interface{})
	err = json.Unmarshal(resBody, &rMap)
	if err != nil {
		return nil, err
	}

	if !rMap["status"].(bool) {
		return nil, errors.New(rMap["error"].(string))
	}

	if len(rMap["rows"].([]interface{})) == 0 {
		return nil, nil
	}

	return rMap["rows"].([]interface{}), nil
}
