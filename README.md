
# ECR Image Cleanup
### Overview

This task has a Go (Golang) script that helps  clean up old, unused container images stored in Amazon ECR (Elastic Container Registry).

Over time, ECR can get cluttered with outdated images that consume space and make it hard to manage repositories. This tool solves that by:

* Scanning all ECR repositories in your AWS account

* Identifying old and untagged images

* Deleting them (or just showing them in dry-run mode)


## Features
* Lists all repositories in a given AWS region
* Checks all images in each repository
* Keeps most 2 recent images that match specific tag prefixes (e.g., latest, dev)
* Deletes images that are older than a specified number of days
* Supports dry-run mode (no actual deletions, just shows what would be deleted)
* Logs output to both the terminal and a log file

## Testing 
For testing purposes in the feature branch, I temporarily changed the retention logic to use minutes instead of days to quickly validate the image cleanup behavior.

We have create one repository named as ``` pbn-assessment``` and in that we have three images in which two tagged as ```latest``` and one tagged as ```dev```.

![Repos](https://github.com/Prerana-Mauryaa/ECR-Cleanup/blob/feature-branch/images/Repos.png)

![Repos](https://github.com/Prerana-Mauryaa/ECR-Cleanup/blob/feature-branch/images/Images.png)

## Running the code

![Repos](https://github.com/Prerana-Mauryaa/ECR-Cleanup/blob/feature-branch/images/gorun.png)

The logs will output the result  to both the terminal and a log file.

You can view the cleanup log here: [ecr-image-cleanup.log](https://github.com/Prerana-Mauryaa/ECR-Cleanup/blob/feature-branch/scripts/ecr-image-cleanup.log)

![Repos](https://github.com/Prerana-Mauryaa/ECR-Cleanup/blob/feature-branch/images/logs.png)


After testing it for multiple cases using minutes as the retention period, we will switch back to using days and perform the tests again.

![Repos](https://github.com/Prerana-Mauryaa/ECR-Cleanup/blob/feature-branch/images/days.png)


## Branching Strategy
We will follow a Feature Branching strategy where each feature is developed in its own isolated branch. These branches are created from the develop branch and are named using the format feature/{feature-name}.

![Repos](https://github.com/Prerana-Mauryaa/ECR-Cleanup/blob/feature-branch/images/Master.png)


## Tagging and Releasing

### Version Tagging:
- Releases are tagged with version numbers following [Semantic Versioning](https://semver.org/) (e.g., `v1.0.0`).
- Tags are created when a feature reaches a stable milestone and is ready for release.

### Release Notes:
Each release will include detailed release notes that describe:
- New features
- Bug fixes
- Any breaking changes

### Release Process:
- Once the feature is merged into the `main` or `production` branch, create a version tag.
- Document the release changes in the `CHANGELOG.md`.
