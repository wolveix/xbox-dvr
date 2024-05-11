package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/wolveix/openxbl-go"
)

func init() {
	cmd.AddCommand(cmdSync)
	cmdSync.AddCommand(cmdSyncClips)
	cmdSync.AddCommand(cmdSyncScreenshots)
}

type dvrContent struct {
	Created   time.Time
	GameTitle string
	ID        string
	Type      string
	URL       string
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

				clips, newContinuationToken, err := client.GetDVRGameClips(continuationToken)
				if err != nil {
					if err.Error() == "failed to find game clips" {
						log.Info().Msgf("No new game clips to download")
						return
					}
					log.Fatal().Err(err).Msg("Failed to retrieve game clips")
				}

				continuationToken = newContinuationToken

				for _, clip := range clips {
					for _, contentLocator := range clip.ContentLocators {
						if contentLocator.LocatorType == "Download" {
							if err := processDVR(client, httpClient, dvrContent{
								Created:   clip.UploadDate,
								GameTitle: clip.TitleName,
								ID:        clip.ContentID,
								Type:      "clips",
								URL:       contentLocator.Uri,
							}); err != nil {
								log.Error().Err(err).Msgf("Failed to process clip: %s", contentLocator.Uri)
							}
						}
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
					if err.Error() == "failed to find game screenshots" {
						log.Info().Msgf("No new game screenshots to download")
						return
					}
					log.Fatal().Err(err).Msg("Failed to retrieve game screenshots")
				}

				continuationToken = newContinuationToken

				for _, screenshot := range screenshots {
					for _, contentLocator := range screenshot.ContentLocators {
						if contentLocator.LocatorType == "Download" {
							if err := processDVR(client, httpClient, dvrContent{
								Created:   screenshot.DateUploaded,
								GameTitle: screenshot.TitleName,
								ID:        screenshot.ContentID,
								Type:      "screenshots",
								URL:       contentLocator.Uri,
							}); err != nil {
								log.Error().Err(err).Msgf("Failed to process screenshot: %s", contentLocator.Uri)
							}
						}
					}
				}

				if continuationToken == "" {
					break
				}
			}
		},
	}
)

func processDVR(client *openxbl.Client, httpClient *http.Client, content dvrContent) error {
	gamePath := filepath.Clean(cfg.SavePath + slash + content.Type + slash + content.GameTitle)
	contentPath := filepath.Clean(gamePath + slash + content.GameTitle + " - " + content.Created.Format("2006-01-02 15_04_05"))

	if content.Type == "clips" {
		contentPath += ".mp4"
	} else {
		contentPath += ".png"
	}

	// create dir for title
	if err := os.MkdirAll(gamePath, os.ModePerm); err != nil {
		log.Fatal().Err(err).Msgf("Failed to create save directory for game: %s", content.GameTitle)
	}

	if _, err := os.Stat(contentPath); err == nil {
		log.Info().Msgf("Skipping %s (already downloaded)", contentPath)
		return nil
	}

	log.Info().Msgf("Downloading %s", contentPath)

	// download file
	response, err := httpClient.Get(content.URL)
	if err != nil {
		return fmt.Errorf("failed to download file: %s", content.URL)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return fmt.Errorf("failed to download file: %s", content.URL)
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

	if cfg.AutoDelete && content.Type == "clips" {
		log.Info().Msg("Deleting clip from XBL")

		if err = client.DeleteDVRGameClip(content.ID); err != nil {
			log.Fatal().Err(err).Msgf("Failed to delete clip: %s", content.URL)
		}
	}

	return nil
}
