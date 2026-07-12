package main

import (

	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"io"
	"fmt"
	"os"
	"os/exec"
	"strings"
	pulEcr "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ecr" 
	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker" //nolint
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"      //nolint
)

func hashDir(root string) (string, error) {
    h := sha256.New()

    err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
        // If there's an issue accessing a path, log it and move on
        if err != nil {
            fmt.Printf("Info: skipping path %s due to access error: %v\n", path, err)
            return nil 
        }

        // Skip the .venv directory entirely
        if d.IsDir() && d.Name() == ".venv" {
            return filepath.SkipDir
        }

        // Only process files
        if !d.IsDir() {
            ext := strings.ToLower(filepath.Ext(path))
            
            // Filter for .py and .txt
            if ext == ".py" || ext == ".txt" || ext == ".html" || ext == ".css" {
                f, err := os.Open(path)
                if err != nil {
                    fmt.Printf("Info: could not open %s: %v\n", path, err)
                    return nil
                }
                defer f.Close()

                if _, err := io.Copy(h, f); err != nil {
                    return err
                }
            }
        }

        return nil
    })

    if err != nil {
        return "", err
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}


func checkIfHashExists(ctx *pulumi.Context, repoURL string, currentHash string) (bool, error) {
	// This code simply checks if *code* has changed on particular repo.
	// Does not account for changes is *not* a hash of the images themsevles.
	parts := strings.Split(repoURL, "/")
    repoName := parts[len(parts)-1]

    _, err := pulEcr.GetImage(ctx, &pulEcr.GetImageArgs{
        RepositoryName: repoName,
        ImageTag:       pulumi.StringRef(currentHash), // Search for the specific hash tag
    }, nil)

    if err != nil {
        // If it returns an error, the tag doesn't exist
        return false, nil
    }

    // If no error, the image with this hash is already in ECR
    return true, nil
}


func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// 1. Retrieve the ECR Repository URL from the environment variable
		ecrRepoUrl := os.Getenv("ECR_WEB_REPO")
		if ecrRepoUrl == "" {
			return fmt.Errorf("ECR_WEB_REPO environment variable is not set")
		}

		// 2. Authenticate Docker with ECR before building/pushing
		// Extract the server URL from the ECR repository URL (domain only, without the repo path)
		// ECR URL format: account.dkr.ecr.region.amazonaws.com/repo-name
		ecrServer := ecrRepoUrl
		if idx := strings.LastIndex(ecrRepoUrl, "/"); idx != -1 {
			ecrServer = ecrRepoUrl[:idx]
		}

		// Get AWS region
		awsRegion := os.Getenv("AWS_REGION")
		if awsRegion == "" {
			awsRegion = "us-west-2"
		}

		// Authenticate Docker with ECR using AWS CLI
		// This runs: aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin <ecr-server>
		ctx.Log.Info(fmt.Sprintf("Authenticating Docker with ECR: %s", ecrServer), nil)
		
		// Get ECR password
		getPasswordCmd := exec.Command("aws", "ecr", "get-login-password", "--region", awsRegion)
		getPasswordCmd.Env = os.Environ()
		passwordOutput, err := getPasswordCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get ECR login password: %w. Make sure AWS CLI is installed and AWS credentials are configured", err)
		}
		password := strings.TrimSpace(string(passwordOutput))

		// Login to Docker registry
		dockerLoginCmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", ecrServer)
		dockerLoginCmd.Env = os.Environ()
		dockerLoginCmd.Stdin = strings.NewReader(password)
		dockerLoginCmd.Stdout = os.Stdout
		dockerLoginCmd.Stderr = os.Stderr
		if err := dockerLoginCmd.Run(); err != nil {
			return fmt.Errorf("failed to login to ECR: %w", err)
		}

		ctx.Log.Info("Successfully authenticated with ECR", nil)

		// 3. Create Docker provider for building and pushing images
		// The Docker provider needs access to the Docker daemon (mounted via docker socket)
		dockerProvider, err := docker.NewProvider(ctx, "docker-provider", &docker.ProviderArgs{
			// When running in Docker, the provider will use the mounted Docker socket
		})
		if err != nil {
			return err
		}

		// 4. Calculate hash of src-test directory to detect changes
		currentHash, err := hashDir("/proj/src")
		if err != nil {
			return fmt.Errorf("failed to hash directory: %w", err)
		}
		ctx.Log.Info(fmt.Sprintf("Current directory hash: %s", currentHash), nil)

		// 5. Only build if no hash existed prior
		// Note: After first successful build, set the hash:
		var image *docker.Image
		hashExists, err := checkIfHashExists(ctx, ecrRepoUrl, currentHash)
		if err == nil {
			if hashExists == false {
			// Build and Push the Docker image from src-test folder
			// Store hash as build argument for reference
			ctx.Log.Info("naming image: " + fmt.Sprintf("%s:%s", ecrRepoUrl, currentHash), nil)
			image, err = docker.NewImage(ctx, "meme-generator-app", &docker.ImageArgs{
				Build: &docker.DockerBuildArgs{
					Context: pulumi.String("src"), // Path relative to working directory /proj
					Args: pulumi.StringMap{
						"SOURCE_HASH": pulumi.String(currentHash),
					},
				},
				ImageName: pulumi.String(fmt.Sprintf("%s:%s", ecrRepoUrl, currentHash)),
				Registry: &docker.RegistryArgs{
					Server: pulumi.String(ecrServer),
				},
			}, pulumi.Provider(dockerProvider))
			if err != nil {
				return err
			}

			ctx.Log.Info("Image built and pushed successfully", nil)
			} else {
				ctx.Log.Info(fmt.Sprintf("Hash unchanged (%s) - skipping build", currentHash), nil)
				// Export that we skipped, but still export the image name for reference
				ctx.Export("imageName", pulumi.String(ecrRepoUrl+":"+currentHash))
				ctx.Export("ecrRepoUrl", pulumi.String(ecrRepoUrl))
				ctx.Export("sourceHash", pulumi.String(currentHash))
				ctx.Export("skipped", pulumi.Bool(true))
				return nil
			}
		}else{
			// we had some kind of error in getting the image from URL
			return err
		}

		// Export the image name and hash
		ctx.Export("imageName", image.ImageName)
		ctx.Export("ecrRepoUrl", pulumi.String(ecrRepoUrl))
		ctx.Export("sourceHash", pulumi.String(currentHash))
		ctx.Export("skipped", pulumi.Bool(false))

		return nil
	})
}

