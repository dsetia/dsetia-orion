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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
	"updater/core"
)

type CmdArguments struct {
	config  string
	daemon  bool
	verbose bool
}

func ParseArgs() CmdArguments {
	// Define flags and their default values.
	configPtr := flag.String("config", "/opt/hndr/etc/updater.json", "Path to config file")
	daemonPtr := flag.Bool("deamon", false, "Run in foreground")
	verbosePtr := flag.Bool("verbose", false, "Enable verbose mode")

	// Parse the command-line arguments.
	flag.Parse()

	cmdArgs := CmdArguments{config: *configPtr, daemon: *daemonPtr, verbose: *verbosePtr}
	return cmdArgs
}

func CheckUpdates(updaterCfg core.UpdaterConfig, snrCfg core.SensorConfig) {
	log.Println("Updater config: ", updaterCfg)
	log.Println("Sensor config: ", snrCfg)

	apiClient := &core.Client{
		BaseURL:  updaterCfg.APIServerURL,
		APIKey:   snrCfg.ApiKey,
		DeviceID: snrCfg.DeviceID,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	defer apiClient.HTTPClient.CloseIdleConnections()

	log.Println("Authenticating...")
	if err := apiClient.Authenticate(snrCfg.TenantID); err != nil {
		log.Println("Auth failed:", err)
		return
	}
	log.Println("Authenticated successfully.")

	// Read the existing update config
	var updtCfg core.UpdateRequest
	if err := core.LoadJSONConfig(updaterCfg.HndrConfig, &updtCfg); err != nil {
		return
	}
	log.Println("Updater config: ", updtCfg)

	// Make request to update service with existing config
	log.Println("Fetching update manifest...")
	update, err := apiClient.GetUpdateManifest(snrCfg.TenantID, updtCfg)
	if err != nil {
		log.Println("Error fetching updates:", err)
		return
	}
	log.Printf("Update manifest: %+v\n", update)
	if len(update.Software.DownloadURL) != 0 { //|| len(update.Rules.DownloadURL) != 0 {
		log.Printf("url: %s\n", update.Software.DownloadURL)

		log.Println("Fetching software...")
		content, err := apiClient.DownloadFile(update.Software.DownloadURL)
		if err != nil {
			log.Println("Error downloading software :", err)
			return
		}

		status, err := core.UpateSWNow(content, update.Software.Version, update.Software.DownloadURL, updaterCfg)
		if err != nil {
			log.Println("Error updating software :", err)
			return
		}

		log.Println("Sending status update...")
		err = apiClient.SendStatus(snrCfg.TenantID, status)
		if err != nil {
			log.Println("Failed to send status:", err)
			return
		}
		log.Println("Status update sent successfully.")
	} else {
		log.Println("Nothing to update...")
	}
}

func main() {
	cmdArgs := ParseArgs()

	// Use the arguments.
	log.Println("Config:", cmdArgs.config)
	log.Println("Daemon:", cmdArgs.daemon)
	log.Println("Verbose:", cmdArgs.verbose)

	var updaterCfg core.UpdaterConfig
	if err := core.LoadJSONConfig(cmdArgs.config, &updaterCfg); err != nil {
		os.Exit(1)
	}
	log.Println("Updater config: ", updaterCfg)

	var snrCfg core.SensorConfig
	if err := core.LoadJSONConfig(updaterCfg.SensorConfig, &snrCfg); err != nil {
		os.Exit(1)
	}
	log.Println("Sensor config: ", snrCfg)

	ctx := context.Background()

	// trap Ctrl+C and call cancel on the context
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	defer func() {
		signal.Stop(c)
		cancel()
	}()

	go func() {
		select {
		case <-c:
			log.Println("Interrupt received, wait for process to complete before exiting")
			cancel()
		case <-ctx.Done():
		}
	}()

	duration := time.Duration(updaterCfg.PollIntervalMins) * time.Minute
	log.Printf("Poll interval in %+vm", duration)
	// Run update once before entering poll interval
	CheckUpdates(updaterCfg, snrCfg)
	for {
		select {
		case <-time.After(duration):
			CheckUpdates(updaterCfg, snrCfg)
		case <-ctx.Done():
			log.Println("We're done here!")
			os.Exit(0)
		}
	}
}
