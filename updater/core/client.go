// Copyright (c) 2025 SecurITe
// All rights reserved.
//
// This source code is the property of SecurITe.
// Unauthorized copying, modification, or distribution of this file,
// via any medium is strictly prohibited unless explicitly authorized
// by SecurITe.
//
// This software is proprietary and confidential.
//
// File Owner:       sumanth@securite.world
// Created On:       04/23/2025

package core

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
)

type Client struct {
    BaseURL    string
    APIKey     string
    DeviceID   string
    HTTPClient *http.Client
}

func (c *Client) sendRequest(method, url string, body any) (*http.Response, error) {
    log.Printf(">>> HTTP Request [%s] %s", method, url)

    var bodyBuf io.Reader
    if body != nil {
        jsonData, err := json.Marshal(body)
        if err != nil {
            return nil, err
        }
        log.Println(">>> Request Body >>>")
        log.Println(string(jsonData))
        log.Println("<<< End Request Body <<<")
        bodyBuf = bytes.NewBuffer(jsonData)
    }

    req, err := http.NewRequest(method, url, bodyBuf)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Content-Type", "application/json")
    if c.APIKey != "" {
        req.Header.Set("X-API-KEY", c.APIKey)
    }
    if c.DeviceID != "" {
        req.Header.Set("X-DEVICE-ID", c.DeviceID)
    }

    log.Println(">>> Headers >>>")
    for k, v := range req.Header {
        log.Printf("%s: %s", k, v)
    }
    log.Println("<<< End Headers <<<")

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return nil, err
    }
    log.Println(">>> Response Headers >>>")
    for k, v := range resp.Header {
        log.Printf("%s: %s", k, v)
    }
    log.Println("<<< End Response Headers <<<")
    return resp, nil
}

func (c *Client) GetUpdateManifest(tenantID string, data UpdateRequest) (UpdateResponse, error) {
    url := fmt.Sprintf("%s/v1/updates/%s", c.BaseURL, tenantID)
    log.Printf("Sending request to URL: %s", url)

    resp, err := c.sendRequest(http.MethodPost, url, data)
    if err != nil {
        log.Printf("Request failed: %v", err)
        return UpdateResponse{}, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, err := io.ReadAll(resp.Body)
        if err != nil {
            return UpdateResponse{}, err
        }
        log.Printf("!OK: Response body size: %d bytes", len(body))
        return UpdateResponse{}, fmt.Errorf("server returned %s: %s", resp.Status, string(body))
    }

    var result UpdateResponse
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("Failed to read response body: %v", err)
        return result, err
    }

    log.Printf("Response body size: %d bytes", len(body))

    err = json.Unmarshal(body, &result)
    if err != nil {
        log.Printf("Failed to unmarshal response body: %v", err)
        return result, err
    }

    log.Printf("Response body (JSON): %+v", result)
    return result, nil
}

func (c *Client) DownloadFile(filePath string) ([]byte, error) {
    url := fmt.Sprintf("%s%s", c.BaseURL, filePath)
    resp, err := c.sendRequest(http.MethodGet, url, nil)
    if err != nil {
        return nil, err
    }

    // Ensure that the response body is closed after reading
    defer resp.Body.Close()

    // Check if the response status is OK
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to download file: %s", resp.Status)
    }

    // Read the entire response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }
    log.Printf("Response body size: %d bytes", len(body))

    return body, nil
}

func (c *Client) Authenticate(tenantID string) error {
    url := fmt.Sprintf("%s/v1/authenticate/%s", c.BaseURL, tenantID)
    resp, err := c.sendRequest(http.MethodGet, url, nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("authentication failed: %s", resp.Status)
    }
    return nil
}

func (c *Client) SendStatus(tenantID string, data StatusRequest) error {
    url := fmt.Sprintf("%s/v1/status/%s", c.BaseURL, tenantID)
    resp, err := c.sendRequest(http.MethodPost, url, data)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("status update failed: %s", resp.Status)
    }
    return nil
}
