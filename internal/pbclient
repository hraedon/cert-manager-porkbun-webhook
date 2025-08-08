package pbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://api.porkbun.com/api/json/v3"

type Client struct {
	http        *http.Client
	APIKey      string
	SecretKey   string
}

func New(apiKey, secretKey string) *Client {
	return &Client{
		http: &http.Client{ Timeout: 30 * time.Second },
		APIKey: apiKey,
		SecretKey: secretKey,
	}
}

type Record struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`   // Porkbun returns FQDN here
	Type    string `json:"type,omitempty"`
	Content string `json:"content,omitempty"`
	TTL     string `json:"ttl,omitempty"`
}

type retrieveResp struct {
	Status  string   `json:"status"`
	Records []Record `json:"records"`
}

type createResp struct {
	Status string `json:"status"`
	ID     string `json:"id"`
}

func (c *Client) post(path string, payload any, out any) error {
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("porkbun %s: %s", resp.Status, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode error: %w; body=%s", err, string(body))
		}
	}
	return nil
}

func (c *Client) RetrieveRecords(ctx context.Context, zone string) ([]Record, error) {
	var out retrieveResp
	err := c.post("/dns/retrieve/"+zone, map[string]string{
		"apikey": c.APIKey, "secretapikey": c.SecretKey,
	}, &out)
	if err != nil { return nil, err }
	if out.Status != "SUCCESS" {
		return nil, fmt.Errorf("retrieve returned status=%s", out.Status)
	}
	return out.Records, nil
}

func (c *Client) CreateTXT(ctx context.Context, zone, fqdn, value, ttl string) (string, error) {
	var out createResp
	// IMPORTANT: Porkbun expects 'name' to be the host portion or FQDN.
	// Using FQDN works reliably and matches what Retrieve returns.
	err := c.post("/dns/create/"+zone, map[string]string{
		"apikey": c.APIKey, "secretapikey": c.SecretKey,
		"type": "TXT", "name": fqdn, "content": value, "ttl": ttl,
	}, &out)
	if err != nil { return "", err }
	if out.Status != "SUCCESS" {
		return "", fmt.Errorf("create returned status=%s", out.Status)
	}
	return out.ID, nil
}

func (c *Client) DeleteRecord(ctx context.Context, zone, id string) error {
	return c.post("/dns/delete/"+zone+"/"+id, map[string]string{
		"apikey": c.APIKey, "secretapikey": c.SecretKey,
	}, nil)
}
