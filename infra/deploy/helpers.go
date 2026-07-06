package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pulEcr "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ecr"
	k8syaml "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type AppRuntimeEnv struct {
	S3Bucket     string
	SourceRegion string
	DynamoTable  string
	LogGroup     string
	CDN          pulumi.StringInput
}

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

// deploymentImageTransform sets the container image on Deployment resources loaded from k8s/*.yaml.
func deploymentImageTransform(imageURL string) k8syaml.Transformation {
	return func(state map[string]interface{}, _ ...pulumi.ResourceOption) {
		if state["kind"] != "Deployment" {
			return
		}

		spec, ok := state["spec"].(map[string]interface{})
		if !ok {
			return
		}
		podTemplate, ok := spec["template"].(map[string]interface{})
		if !ok {
			return
		}
		podSpec, ok := podTemplate["spec"].(map[string]interface{})
		if !ok {
			return
		}
		containers, ok := podSpec["containers"].([]interface{})
		if !ok || len(containers) == 0 {
			return
		}
		container, ok := containers[0].(map[string]interface{})
		if !ok {
			return
		}

		container["image"] = imageURL
	}
}

// deploymentEnvTransform injects runtime environment variables into the app Deployment.
func deploymentEnvTransform(appEnv AppRuntimeEnv) k8syaml.Transformation {
	return func(state map[string]interface{}, _ ...pulumi.ResourceOption) {
		if state["kind"] != "Deployment" {
			return
		}

		spec, ok := state["spec"].(map[string]interface{})
		if !ok {
			return
		}
		podTemplate, ok := spec["template"].(map[string]interface{})
		if !ok {
			return
		}
		podSpec, ok := podTemplate["spec"].(map[string]interface{})
		if !ok {
			return
		}
		containers, ok := podSpec["containers"].([]interface{})
		if !ok || len(containers) == 0 {
			return
		}
		container, ok := containers[0].(map[string]interface{})
		if !ok {
			return
		}

		container["env"] = []interface{}{
			map[string]interface{}{"name": "S3_BUCKET", "value": appEnv.S3Bucket},
			map[string]interface{}{"name": "SOURCE_REGION", "value": appEnv.SourceRegion},
			map[string]interface{}{"name": "DYNAMO_TABLE", "value": appEnv.DynamoTable},
			map[string]interface{}{"name": "CDN", "value": appEnv.CDN},
			map[string]interface{}{"name": "LOG_GROUP", "value": appEnv.LogGroup},
		}
	}
}

// loadBalancerSubnetTransform pins the NLB to the public subnets from the network stack.
func loadBalancerSubnetTransform(publicSubnetA, publicSubnetB string) k8syaml.Transformation {
	return func(state map[string]interface{}, _ ...pulumi.ResourceOption) {
		if state["kind"] != "Service" {
			return
		}
		meta, ok := state["metadata"].(map[string]interface{})
		if !ok {
			return
		}
		annotations, ok := meta["annotations"].(map[string]interface{})
		if !ok {
			annotations = map[string]interface{}{}
			meta["annotations"] = annotations
		}
		annotations["service.beta.kubernetes.io/aws-load-balancer-subnets"] = fmt.Sprintf("%s,%s", publicSubnetA, publicSubnetB)
	}
}
