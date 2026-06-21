package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"io"
	"os"
	"strings"
	pulEcr "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ecr"
	"fmt"
	k8syaml "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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

// targetGroupBindingTransform sets the target group ARN on TargetGroupBinding resources.
func targetGroupBindingTransform(targetGroupARN string) k8syaml.Transformation {
	return func(state map[string]interface{}, _ ...pulumi.ResourceOption) {
		if state["kind"] != "TargetGroupBinding" {
			return
		}
		spec, ok := state["spec"].(map[string]interface{})
		if !ok {
			return
		}
		spec["targetGroupARN"] = targetGroupARN
	}
}

// albControllerInstallTransform patches the AWS Load Balancer Controller install manifest
// with the EKS cluster name, AWS region, and IRSA role ARN.
func albControllerInstallTransform(clusterName, awsRegion, roleArn string) k8syaml.Transformation {
	return func(state map[string]interface{}, _ ...pulumi.ResourceOption) {
		kind, _ := state["kind"].(string)
		meta, ok := state["metadata"].(map[string]interface{})
		if !ok {
			return
		}
		name, _ := meta["name"].(string)
		namespace, _ := meta["namespace"].(string)

		if kind == "ServiceAccount" && name == "aws-load-balancer-controller" && namespace == "kube-system" {
			annotations, ok := meta["annotations"].(map[string]interface{})
			if !ok {
				annotations = map[string]interface{}{}
				meta["annotations"] = annotations
			}
			annotations["eks.amazonaws.com/role-arn"] = roleArn
			return
		}

		if kind != "Deployment" || name != "aws-load-balancer-controller" || namespace != "kube-system" {
			return
		}
		spec, ok := state["spec"].(map[string]interface{})
		if !ok {
			return
		}
		template, ok := spec["template"].(map[string]interface{})
		if !ok {
			return
		}
		podSpec, ok := template["spec"].(map[string]interface{})
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
		args, ok := container["args"].([]interface{})
		if !ok {
			args = []interface{}{}
		}
		newArgs := make([]interface{}, 0, len(args)+2)
		for _, a := range args {
			s, _ := a.(string)
			if strings.HasPrefix(s, "--cluster-name=") || strings.HasPrefix(s, "--aws-region=") {
				continue
			}
			newArgs = append(newArgs, a)
		}
		newArgs = append(newArgs,
			fmt.Sprintf("--cluster-name=%s", clusterName),
			fmt.Sprintf("--aws-region=%s", awsRegion),
		)
		container["args"] = newArgs
	}
}