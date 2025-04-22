package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

func main() {
	// Setup structured logging to both stdout and file
	logFile, err := os.OpenFile("ecr-cleanup.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("❌ Failed to open log file: %v", err)
	}
	defer logFile.Close()

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Input variables
	var region string
	var prefixList string
	var dryRunInput string
	var dryRun bool

	// Get user inputs
	fmt.Print("Enter AWS Region (e.g., us-east-1): ")
	fmt.Scanln(&region)

	fmt.Print("Enter comma-separated tag prefixes to retain (e.g., latest,dev,main): ")
	fmt.Scanln(&prefixList)

	fmt.Print("Dry-run mode? (yes/no): ")
	fmt.Scanln(&dryRunInput)
	dryRun = strings.ToLower(dryRunInput) == "yes"

	log.Printf("📌 Region: %s | Prefixes: %s | Dry-run: %v", region, prefixList, dryRun)

	// AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		log.Fatalf("❌ Error creating session: %v", err)
	}

	// ECR client
	svc := ecr.New(sess)

	// List repositories
	repos, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{})
	if err != nil {
		log.Fatalf("❌ Failed to list repositories: %v", err)
	}

	if len(repos.Repositories) == 0 {
		log.Println("⚠️ No repositories found in the specified region.")
		return
	}

	prefixes := strings.Split(prefixList, ",")

	// Process each repository
	for _, repo := range repos.Repositories {
		repoName := *repo.RepositoryName
		log.Printf("\n📦 Repository: %s", repoName)

		imageOutput, err := svc.DescribeImages(&ecr.DescribeImagesInput{
			RepositoryName: aws.String(repoName),
		})
		if err != nil {
			log.Printf("⚠️ Error fetching images for %s: %v", repoName, err)
			continue
		}

		if len(imageOutput.ImageDetails) == 0 {
			log.Printf("⚠️ No images found in repository %s", repoName)
			continue
		}

		var taggedMatches []*ecr.ImageDetail
		var untagged []*ecr.ImageDetail

		for _, img := range imageOutput.ImageDetails {
			if img.ImagePushedAt == nil {
				continue
			}

			if len(img.ImageTags) == 0 {
				untagged = append(untagged, img)
				continue
			}

			matched := false
			for _, tag := range img.ImageTags {
				for _, prefix := range prefixes {
					if strings.HasPrefix(*tag, prefix) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}

			if matched {
				taggedMatches = append(taggedMatches, img)
			}
		}

		// Sort matched images by pushed time (descending)
		sort.Slice(taggedMatches, func(i, j int) bool {
			return taggedMatches[i].ImagePushedAt.After(*taggedMatches[j].ImagePushedAt)
		})

		// Retain only the latest 2
		keepMap := make(map[string]bool)
		for i, img := range taggedMatches {
			if i < 2 {
				log.Printf("✅ Retaining image (recent tag): %s", *img.ImageDigest)
				keepMap[*img.ImageDigest] = true
			}
		}

		// Collect images to delete: older tagged + untagged
		var toDelete []*ecr.ImageDetail
		for _, img := range taggedMatches {
			if _, keep := keepMap[*img.ImageDigest]; !keep {
				toDelete = append(toDelete, img)
			}
		}
		toDelete = append(toDelete, untagged...)

		// Delete or dry-run
		for _, img := range toDelete {
			digest := *img.ImageDigest
			if !dryRun {
				_, err := svc.BatchDeleteImage(&ecr.BatchDeleteImageInput{
					RepositoryName: aws.String(repoName),
					ImageIds: []*ecr.ImageIdentifier{
						{ImageDigest: aws.String(digest)},
					},
				})
				if err != nil {
					log.Printf("❌ Error deleting image %s: %v", digest, err)
				} else {
					log.Printf("🗑️ Deleted image: %s", digest)
				}
			} else {
				log.Printf("ℹ️ Dry-run: Would delete image %s", digest)
			}
		}
	}
}
