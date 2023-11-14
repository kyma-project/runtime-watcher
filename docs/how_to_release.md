# Runtime Watcher Release Procedure

Use the steps described in this document to release a new version of Runtime Watcher.

## Steps

1. Checkout the main branch.
2. Create a new tag for the release in the main branch and push it to the repository. The tag should follow the [Semantic Versioning 2.0.0 format](https://semver.org/), for example, `1.2.3`.
   The new tag will trigger a ProwJob that creates and publishes a Docker image with a Runtime Watcher executable.
   The new tag will also trigger the release workflow.

> **NOTE:** The release workflow waits for a Docker image to be published. If the image is not available within the configured time (15 minutes), the workflow fails. In that case, you need to manually re-run it once the Docker image is ready. The expected image URL is: `europe-docker.pkg.dev/kyma-project/prod/runtime-watcher-skr:<NewTag>`

> **NOTE:** If you want to create a tag without triggering a release, just ensure the tag does not match the Semantic Versioning format.

