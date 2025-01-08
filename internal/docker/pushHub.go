package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"github.com/pterm/pterm"
)


// PushImage pushes the specified Docker image to the Docker Hub.
// It authenticates with Docker Hub, tags the image, and pushes it to the registry.
// It displays a spinner with progress updates and prints the push response messages.
func PushImage(opts PushOptions) error {
	spinner, _ := pterm.DefaultSpinner.Start("Initializing Docker client...")

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		spinner.Fail("Failed to initialize Docker client")
		return fmt.Errorf("docker client init failed: %w", err)
	}
	defer cli.Close()

	spinner.UpdateText("Preparing authentication...")

	authConfig := registry.AuthConfig{
		Username: os.Getenv("DOCKER_USERNAME"),
		Password: os.Getenv("DOCKER_PASSWORD"),
	}

	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		spinner.Fail("Authentication preparation failed")
		return fmt.Errorf("auth encoding failed: %w", err)
	}

	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	spinner.UpdateText(fmt.Sprintf("Pushing image %s...", opts.ImageName))

	pushResp, err := cli.ImagePush(ctx, opts.ImageName, image.PushOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to push image %s", opts.ImageName))
		return fmt.Errorf("push failed: %w", err)
	}
	defer pushResp.Close()

	decoder := json.NewDecoder(pushResp)
	progressBar, _ := pterm.DefaultProgressbar.WithTotal(100).WithTitle("Push Progress").Start()

	for {
		var msg jsonMessage
		if err := decoder.Decode(&msg); err != nil {
			break
		}

		if msg.Error != "" {
			spinner.Fail(msg.Error)
			return fmt.Errorf("push error: %s", msg.Error)
		}

		if msg.Status != "" {
			if strings.Contains(msg.Status, "Layer already exists") {
				progressBar.Add(10)
			} else if strings.Contains(msg.Status, "Pushed") {
				progressBar.Add(15)
			}

			spinner.UpdateText(msg.Status)
		}

		if msg.Progress != "" {
			progressBar.UpdateTitle(fmt.Sprintf("%s: %s", msg.Status, msg.Progress))
		}
	}

	progressBar.Stop()
	spinner.Success(color.GreenString("Successfully pushed image %s", opts.ImageName))

	return nil
}

