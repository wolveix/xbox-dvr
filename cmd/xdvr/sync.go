package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wolveix/openxbl-go"
)

func init() {
	cmd.AddCommand(cmdSync)
	cmdSync.AddCommand(cmdSyncClips)
	cmdSync.AddCommand(cmdSyncScreenshots)
}

var (
	cmdSync = &cobra.Command{
		Use:   "sync",
		Short: "Sync your latest DVR clips and screenshots",
		Args:  cobra.ExactArgs(0),
		Run: func(command *cobra.Command, args []string) {
			cmdSyncClips.Run(command, args)
			cmdSyncScreenshots.Run(command, args)
		},
	}

	cmdSyncClips = &cobra.Command{
		Use:   "clips",
		Short: "Sync your latest DVR clips",
		Run: func(command *cobra.Command, args []string) {
			if cfg.APIKey == "" {
				log.Fatal().Msg("API key is required, set it with `xdvr config set apiKey your-api-key`")
			}

			client := openxbl.NewClient(cfg.APIKey, timeout)
			httpClient := &http.Client{Timeout: timeout}

			var continuationToken string

			for {
				log.Info().Msgf("Finding DVR clips")

				clips, newContinuationToken, err := client.GetDVRClips(continuationToken)
				if err != nil {
					if err.Error() == "failed to find clips" {
						log.Info().Msgf("No new clips to download")
						return
					}
					log.Fatal().Err(err).Msg("Failed to retrieve clips")
				}

				continuationToken = newContinuationToken

				for _, clip := range clips {
					downloadLink := clip.GetDownloadLink()
					if downloadLink == "" {
						continue
					}

					if err := processDVR(client, httpClient, clip.DVRCapture); err != nil {
						log.Error().Err(err).Msgf("Failed to process clip: %s", downloadLink)
					}
				}

				if continuationToken == "" {
					break
				}
			}
		},
	}

	cmdSyncScreenshots = &cobra.Command{
		Use:   "screenshots",
		Short: "Sync your latest DVR screenshots",
		Run: func(command *cobra.Command, args []string) {
			if cfg.APIKey == "" {
				log.Fatal().Msg("API key is required, set it with `xdvr config set apiKey your-api-key`")
			}

			if cfg.AutoDelete {
				log.Warn().Msg("Auto delete enabled, but screenshots can't be automatically deleted")
			}

			client := openxbl.NewClient(cfg.APIKey, timeout)
			httpClient := &http.Client{Timeout: timeout}

			var continuationToken string

			for {
				log.Info().Msgf("Finding DVR screenshots")

				screenshots, newContinuationToken, err := client.GetDVRScreenshots(continuationToken)
				if err != nil {
					if err.Error() == "failed to find screenshots" {
						log.Info().Msgf("No new screenshots to download")
						return
					}
					log.Fatal().Err(err).Msg("Failed to retrieve screenshots")
				}

				continuationToken = newContinuationToken

				for _, screenshot := range screenshots {
					downloadLink := screenshot.GetDownloadLink()
					if downloadLink == "" {
						continue
					}

					if err := processDVR(client, httpClient, screenshot.DVRCapture); err != nil {
						log.Error().Err(err).Msgf("Failed to process screenshot: %s", downloadLink)
					}
				}

				if continuationToken == "" {
					break
				}
			}
		},
	}
)

func processDVR(client *openxbl.Client, httpClient *http.Client, capture openxbl.DVRCapture) error {
	downloadURL := capture.GetDownloadLink()
	if downloadURL == "" {
		return nil
	}

	gamePath := filepath.Clean(cfg.SavePath + slash + strings.ToLower(string(capture.Type)) + "s" + slash + capture.TitleName)
	contentPath := filepath.Clean(gamePath + slash + capture.TitleName + " - " + capture.UploadDate.Format("2006-01-02 15_04_05"))

	if capture.Type == openxbl.DVRCaptureTypeClip {
		contentPath += ".mp4"
	} else {
		contentPath += ".png"
	}

	// create dir for title
	if err := os.MkdirAll(gamePath, os.ModePerm); err != nil {
		log.Fatal().Err(err).Msgf("Failed to create save directory for game: %s", capture.TitleName)
	}

	if _, err := os.Stat(contentPath); err == nil {
		log.Info().Msgf("Skipping %s (already downloaded)", contentPath)
		return nil
	}

	log.Info().Msgf("Downloading %s", contentPath)

	// download file
	response, err := httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download file: %s", downloadURL)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return fmt.Errorf("failed to download file: %s", downloadURL)
	}

	// read response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// write response body to disk
	if err = os.WriteFile(contentPath, body, 0o644); err != nil {
		return fmt.Errorf("failed to write file to disk: %w", err)
	}

	if cfg.AutoDelete && capture.Type == "clips" {
		log.Info().Msg("Deleting clip from XBL")

		if err = client.DeleteDVRClip(capture.ID); err != nil {
			log.Fatal().Err(err).Msgf("Failed to delete clip: %s", downloadURL)
		}
	}

	return nil
}
