package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const (
	DNSEndpoint = "https://dns.hetzner.com/api/v1"
)

type CreateDNSRecordRequest struct {
	ZoneID string `json:"zone_id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	TTL    int    `json:"ttl"`
}

type CreateDNSRecordResponse struct {
	Record struct {
		ID string `json:"id"`
	} `json:"record"`
}

func createDNSRecord(zoneID string, name string, dnsType string, value string) (string, error) {
	createDNSRecordRequest := CreateDNSRecordRequest{
		ZoneID: zoneID,
		Type:   dnsType,
		Name:   name,
		Value:  value,
		TTL:    300,
	}
	body, err := json.Marshal(createDNSRecordRequest)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/records", DNSEndpoint)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Auth-API-Token", os.Getenv("HCLOUD_DNS_TOKEN"))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create dns record %s: %s", name, resp.Status)
	}

	var createDNSRecordResponse CreateDNSRecordResponse
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(respBody, &createDNSRecordResponse)
	if err != nil {
		return "", err
	}
	return createDNSRecordResponse.Record.ID, nil
}

func deleteDNSRecord(recordID string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/records/%s", DNSEndpoint, recordID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Auth-API-Token", os.Getenv("HCLOUD_DNS_TOKEN"))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete dns record %s: %s", recordID, resp.Status)
	}
	return nil
}
