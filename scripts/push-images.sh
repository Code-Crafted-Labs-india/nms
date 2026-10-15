#!/usr/bin/env bash
set -euo pipefail

readonly DOCKERHUB_USER="ritiklrt"
readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

IMAGE_TAG="${1:-}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"

if [[ -z "$IMAGE_TAG" ]]; then
  if ! command -v git >/dev/null 2>&1; then
    echo "Usage: $0 <image-tag> [platforms]" >&2
    exit 2
  fi
  IMAGE_TAG="$(git -C "$ROOT_DIR" rev-parse --short HEAD)"
fi

if [[ ! "$IMAGE_TAG" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
  echo "Invalid image tag: $IMAGE_TAG" >&2
  exit 2
fi
if [[ -n "${2:-}" ]]; then
  PLATFORMS="$2"
fi
if [[ -z "$PLATFORMS" ]]; then
  echo "PLATFORMS must contain at least one Docker platform." >&2
  exit 2
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "Docker CLI is required." >&2
  exit 127
fi

if ! docker buildx version >/dev/null 2>&1; then
  echo "Docker Buildx is required. Install or enable the Docker Buildx plugin." >&2
  exit 127
fi

echo "Pushing NMS images to Docker Hub as ${DOCKERHUB_USER} (tag: ${IMAGE_TAG})."
echo "Platforms: ${PLATFORMS}"
echo "If needed, authenticate first with: docker login --username ${DOCKERHUB_USER}"

docker buildx build \
  --platform "$PLATFORMS" \
  --file "$ROOT_DIR/dockerfile" \
  --tag "${DOCKERHUB_USER}/nms-middleware:${IMAGE_TAG}" \
  --tag "${DOCKERHUB_USER}/nms-middleware:latest" \
  --push \
  "$ROOT_DIR"

docker buildx build \
  --platform "$PLATFORMS" \
  --file "$ROOT_DIR/ui/Dockerfile" \
  --tag "${DOCKERHUB_USER}/nms-ui:${IMAGE_TAG}" \
  --tag "${DOCKERHUB_USER}/nms-ui:latest" \
  --push \
  "$ROOT_DIR/ui"

echo "Images pushed:"
echo "  ${DOCKERHUB_USER}/nms-middleware:${IMAGE_TAG}"
echo "  ${DOCKERHUB_USER}/nms-ui:${IMAGE_TAG}"
