package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

var logger *log.Logger

func setupLogger() {
	logFile, err := os.OpenFile("ecr-image-cleanup.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("❌ Failed to open log file: %v", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger = log.New(multiWriter, "", log.Ldate|log.Ltime)
}

func main() {
	setupLogger()

	var region string
	var retention int
	var prefixList string
	var dryRunInput string
	var dryRun bool

	// Step 1: Ask user for inputs
	fmt.Print("Enter AWS Region (e.g., us-east-1): ")
	fmt.Scanln(&region)

	fmt.Print("Enter retention period in days (e.g., 10): ")
	fmt.Scanln(&retention)

	fmt.Print("Enter comma-separated tag prefixes to keep (e.g., latest,dev,main): ")
	fmt.Scanln(&prefixList)

	fmt.Print("Dry-run mode? (yes/no): ")
	fmt.Scanln(&dryRunInput)
	dryRun = strings.ToLower(dryRunInput) == "yes"

	logger.Printf("[INFO] Starting ECR cleanup in region %s | Retention: %d days | Prefixes: %s | Dry-run: %v",
		region, retention, prefixList, dryRun)

	// Step 2: Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		logger.Fatalf("[ERROR] Error creating AWS session: %v", err)
	}

	// Step 3: Create ECR client
	svc := ecr.New(sess)

	// Step 4: List repositories
	repos, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{})
	if err != nil {
		logger.Fatalf("[ERROR] Failed to list repositories: %v", err)
	}

	if len(repos.Repositories) == 0 {
		logger.Println("[WARNING] No repositories found in the specified region.")
		return
	}

	prefixes := strings.Split(prefixList, ",")

	// Step 5: Loop through each repository
	for _, repo := range repos.Repositories {
		repoName := *repo.RepositoryName
		logger.Printf("\n[INFO] 📦 Processing Repository: %s", repoName)

		// Step 6: Get all images in the repository
		imageOutput, err := svc.DescribeImages(&ecr.DescribeImagesInput{
			RepositoryName: aws.String(repoName),
		})
		if err != nil {
			logger.Printf("[WARNING] Failed to describe images for %s: %v", repoName, err)
			continue
		}

		if len(imageOutput.ImageDetails) == 0 {
			logger.Printf("[INFO] No images found in repository %s", repoName)
			continue
		}

		// Step 7: Prepare for sorting and retention logic
		type taggedImage struct {
			digest     string
			tags       []*string
			pushedTime time.Time
		}

		prefixMatchMap := make(map[string][]taggedImage)

		for _, image := range imageOutput.ImageDetails {
			if image.ImagePushedAt == nil || len(image.ImageTags) == 0 {
				continue
			}
			for _, tag := range image.ImageTags {
				for _, prefix := range prefixes {
					if strings.HasPrefix(*tag, prefix) {
						prefixMatchMap[prefix] = append(prefixMatchMap[prefix], taggedImage{
							digest:     *image.ImageDigest,
							tags:       image.ImageTags,
							pushedTime: *image.ImagePushedAt,
						})
						break
					}
				}
			}
		}

		// Step 8: Build a set of digests to retain (top 2 per prefix)
		retainedDigests := make(map[string]bool)
		for _, images := range prefixMatchMap {
			sort.Slice(images, func(i, j int) bool {
				return images[i].pushedTime.After(images[j].pushedTime)
			})

			for i := 0; i < len(images) && i < 2; i++ {
				retainedDigests[images[i].digest] = true
			}
		}

		// Step 9: Process each image
		for _, image := range imageOutput.ImageDetails {
			if image.ImagePushedAt == nil {
				continue
			}
			imageAge := int(time.Since(*image.ImagePushedAt).Hours() / 24)

			// Untagged images
			if len(image.ImageTags) == 0 {
				logger.Printf("[DELETE] 🗑️ Untagged image candidate: %s", *image.ImageDigest)
				continue
			}

			// Retained?
			if retainedDigests[*image.ImageDigest] {
				logger.Printf("[KEEP] ✅ Image retained (latest tag-match): %s | Tags: %v", *image.ImageDigest, image.ImageTags)
				continue
			}

			// Delete if older than retention
			if imageAge > retention {
				logger.Printf("[DELETE] 🗑️ Old image to delete: %s | Age: %d days | Tags: %v",
					*image.ImageDigest, imageAge, image.ImageTags)

				if !dryRun {
					_, err := svc.BatchDeleteImage(&ecr.BatchDeleteImageInput{
						RepositoryName: aws.String(repoName),
						ImageIds: []*ecr.ImageIdentifier{
							{ImageDigest: image.ImageDigest},
						},
					})
					if err != nil {
						logger.Printf("[ERROR] ❌ Error deleting image %s: %v", *image.ImageDigest, err)
					} else {
						logger.Printf("[SUCCESS] ✅ Image deleted: %s", *image.ImageDigest)
					}
				}
			}
		}
	}

	logger.Println("[INFO] ✅ ECR cleanup completed.")
}
