package opticket

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

	"github.com/scitix/aegis/pkg/ticketmodel"
)

const (
	createTicketPath   string = "/api/v1/support/ticket/create"
	getTicketPath      string = "/api/v1/support/ticket/get"
	patchTicketPath    string = "/api/v1/support/ticket/patch"
	acceptTicketPath   string = "/api/v1/support/ticket/accept"
	dispatchTicketPath string = "/api/v1/support/ticket/dispatch"
	resolveTicketPath  string = "/api/v1/support/ticket/resolve"
	listTicketsPath    string = "/api/v1/support/ticket/list"
	closeTicketPath    string = "/api/v1/support/ticket/close"

	listCESPath string = "/api/v1/ces/instance/list"
)

type OpTicketClient struct {
	endpoint string
}

func CreateOpTicketClient(endpoint string) (*OpTicketClient, error) {
	_, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("Invalid url endpoint %s: %v", endpoint, err)
	}

	return &OpTicketClient{
		endpoint: strings.TrimSuffix(endpoint, "/"),
	}, nil
}

func getHeader() map[string]string {
	appID := os.Getenv("AppID")
	token := os.Getenv("Token")
	ranstr := os.Getenv("Ranstr")

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	strA := token + ranstr + timestamp
	hash := sha256.Sum256([]byte(strA))

	return map[string]string{
		"Signature": hex.EncodeToString(hash[:]),
		"AppID":     appID,
		"Randstr":   ranstr,
		"Timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
}

func (u *OpTicketClient) CreateTicket(ctx context.Context, region, orgName, node, nodeSN, title, description, hardwareType string) error {
	data := map[string]interface{}{
		"region":          region,
		"orgName":         orgName,
		"creator":         "aegis",
		"title":           title,
		"priority":        "high",
		"nodeName":        node,
		"nodeSN":          nodeSN,
		"isHardwareIssue": true,
		"model":           "hardware",
		"hardwareType":    hardwareType,
		"startTime":       time.Now().Format("2006-01-02T15:04:05Z07:00"),
		"description":     description,
		"isFromCustomer":  false,
	}

	headers := getHeader()
	address := u.endpoint + createTicketPath

	return post(ctx, address, data, headers)
}

func (u *OpTicketClient) CreateComponentTicket(ctx context.Context, region, orgName, node, nodeSN, title, description, model, component string) error {
	data := map[string]interface{}{
		"region":          region,
		"orgName":         orgName,
		"creator":         "aegis",
		"title":           title,
		"priority":        "high",
		"node":            node,
		"nodeSN":          nodeSN,
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

func (u *OpTicketClient) PatchTicket(ctx context.Context, ticketId, key, value string) error {
	data := map[string]interface{}{
		"ticketId": ticketId,
		"key":      key,
		"value":    value,
	}

	headers := getHeader()
	address := u.endpoint + patchTicketPath

	return post(ctx, address, data, headers)
}

func (u *OpTicketClient) GetTicket(ctx context.Context, ticketId string) (*OpTicket, error) {
	data := map[string]string{
		"ticketId": ticketId,
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

	info := &OpTicket{
		TicketId:        res["ticketId"].(string),
		Status:          ticketmodel.TicketStatus(res["status"].(string)),
		Region:          res["region"].(string),
		OrgName:         res["orgName"].(string),
		NodeSN:          res["nodeSN"].(string),
		IsHardwareIssue: res["isHardwareIssue"].(bool),
		Creator:         res["creator"].(string),
		Description:     res["description"].(string),
		Supervisor:      res["supervisor"].(string),
		Title:           res["title"].(string),
	}

	return info, nil
}

func (u *OpTicketClient) AcceptTicket(ctx context.Context, ticketId string) error {
	data := map[string]interface{}{
		"ticketID": ticketId,
		"status":   "resolving",
	}

	headers := getHeader()
	address := u.endpoint + acceptTicketPath

	return post(ctx, address, data, headers)
}

func (u *OpTicketClient) DispatchTicket(ctx context.Context, ticketId string, user string) error {
	data := map[string]interface{}{
		"ticketId":   ticketId,
		"supervisor": user,
		"status":     "assigned",
	}

	headers := getHeader()
	address := u.endpoint + dispatchTicketPath

	return post(ctx, address, data, headers)
}

func (u *OpTicketClient) ResolveTicket(ctx context.Context, ticketId, answer, operation string, isHardwareIssue bool) error {
	data := map[string]interface{}{
		"ticketId":        ticketId,
		"answer":          answer,
		"operation":       operation,
		"finishTime":      time.Now().Format("2006-01-02T15:04:05Z07:00"),
		"isHardwareIssue": isHardwareIssue,
	}

	headers := getHeader()
	address := u.endpoint + resolveTicketPath

	return post(ctx, address, data, headers)
}

func (u *OpTicketClient) CloseTicket(ctx context.Context, ticketId string) error {
	data := map[string]interface{}{
		"ticketId": ticketId,
	}

	headers := getHeader()
	address := u.endpoint + closeTicketPath

	return post(ctx, address, data, headers)
}

func (u *OpTicketClient) GetNodeInfo(ctx context.Context, region, orgName, ip string) (*CESInstance, error) {
	data := map[string]string{
		"region":  region,
		"orgName": orgName,
		"ip":      ip,
	}

	headers := getHeader()
	address := u.endpoint + listCESPath

	result, err := get(ctx, address, data, headers)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	res := result.(map[string]interface{})

	info := &CESInstance{
		Region:     res["regionId"].(string),
		OrgName:    res["tenantName"].(string),
		SN:         res["sn"].(string),
		IP:         res["ip"].(string),
		Status:     res["status"].(string),
		InstanceId: res["instanceId"].(string),
	}

	return info, nil
}

func (u *OpTicketClient) GetNodeFirstUnResovledTicket(ctx context.Context, region, nodeSN string) (*OpTicket, error) {
	data := map[string]string{
		"region":   region,
		"nodeSN":   nodeSN,
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

	info := &OpTicket{
		TicketId:        res["ticketId"].(string),
		Status:          ticketmodel.TicketStatus(res["status"].(string)),
		Region:          res["region"].(string),
		OrgName:         res["orgName"].(string),
		NodeSN:          res["nodeSN"].(string),
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

func (u *OpTicketClient) ListNodeTickets(ctx context.Context, region, nodeSN string, size int) ([]*OpTicket, error) {
	data := map[string]string{
		"region":   region,
		"nodeSN":   nodeSN,
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

	res := make([]*OpTicket, 0)
	for _, t := range results {
		r := t.(map[string]interface{})
		info := &OpTicket{
			TicketId:        r["ticketId"].(string),
			Status:          ticketmodel.TicketStatus(r["status"].(string)),
			Region:          r["region"].(string),
			OrgName:         r["orgName"].(string),
			NodeSN:          r["nodeSN"].(string),
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

	if rMap["rows"] == nil || len(rMap["rows"].([]interface{})) == 0 {
		return nil, nil
	}

	return rMap["rows"].([]interface{}), nil
}
