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