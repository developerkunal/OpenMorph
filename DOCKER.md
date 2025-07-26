# OpenMorph Docker Guide

This guide covers how to use OpenMorph in containerized environments, particularly for CI/CD pipelines.

## Quick Start

### Using Pre-built Image (Recommended for CI/CD)

```bash
# Pull the latest image
docker pull ghcr.io/developerkunal/openmorph:latest

# Transform files in current directory
docker run --rm -v $(pwd):/workspace ghcr.io/developerkunal/openmorph:latest \
  --input /workspace --dry-run
```

### Building Locally

```bash
# Build production image
docker build -t openmorph:latest .

# Build development image with shell access
docker build -f Dockerfile.dev -t openmorph:dev .

# Build with distroless (most secure)
docker build -f Dockerfile.distroless -t openmorph:distroless .
```

## Docker Images

### Production Image (`Dockerfile`)

- **Base**: `scratch` (minimal attack surface)
- **Size**: ~10MB
- **Security**: Non-root user, no shell
- **Use case**: Production CI/CD pipelines

### Distroless Image (`Dockerfile.distroless`)

- **Base**: `gcr.io/distroless/static:nonroot`
- **Size**: ~15MB
- **Security**: Enhanced security with distroless
- **Use case**: High-security environments

### Development Image (`Dockerfile.dev`)

- **Base**: `alpine:3.20`
- **Size**: ~50MB
- **Features**: Shell access, debugging tools (jq, yq, vim)
- **Use case**: Development and debugging

## CI/CD Integration

### GitHub Actions

```yaml
name: Transform OpenAPI Specs
on:
  push:
    paths: ["specs/**"]

jobs:
  transform:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Transform OpenAPI specs
        run: |
          docker run --rm \
            -v ${{ github.workspace }}:/workspace \
            -v ${{ github.workspace }}/output:/output \
            ghcr.io/developerkunal/openmorph:latest \
            --input /workspace/specs \
            --config /workspace/openmorph.yaml \
            --output /output/transformed-spec.yaml \
            --backup

      - name: Upload transformed specs
        uses: actions/upload-artifact@v4
        with:
          name: transformed-specs
          path: output/
```

### GitLab CI

```yaml
transform-specs:
  image: ghcr.io/developerkunal/openmorph:latest
  stage: transform
  script:
    - openmorph --input ./specs --config ./openmorph.yaml --output ./output/transformed.yaml
  artifacts:
    paths:
      - output/
    expire_in: 1 week
  only:
    changes:
      - specs/**/*
```

### Jenkins Pipeline

```groovy
pipeline {
    agent any
    stages {
        stage('Transform OpenAPI') {
            steps {
                script {
                    docker.image('ghcr.io/developerkunal/openmorph:latest').inside('-v $WORKSPACE:/workspace') {
                        sh '''
                            openmorph \
                                --input /workspace/specs \
                                --config /workspace/openmorph.yaml \
                                --output /workspace/output/transformed.yaml \
                                --backup
                        '''
                    }
                }
            }
        }
    }
}
```

### Azure DevOps

```yaml
- task: Docker@2
  displayName: "Transform OpenAPI specs"
  inputs:
    command: "run"
    image: "ghcr.io/developerkunal/openmorph:latest"
    arguments: |
      -v $(System.DefaultWorkingDirectory):/workspace
      -v $(System.DefaultWorkingDirectory)/output:/output
    containerCommand: |
      --input /workspace/specs
      --config /workspace/openmorph.yaml
      --output /output/transformed.yaml
      --backup
```

## Docker Compose Usage

### Development

```bash
# Start development environment
docker-compose up openmorph-dev

# This gives you a shell with openmorph available
```

### CI/CD Simulation

```bash
# Run CI/CD profile
docker-compose --profile ci up openmorph-ci
```

### Custom Configuration

```yaml
# docker-compose.override.yml
version: "3.8"
services:
  openmorph:
    command:
      [
        "--input",
        "/workspace/my-specs",
        "--config",
        "/workspace/my-config.yaml",
        "--output",
        "/output/result.yaml",
      ]
```

## Volume Mounting

### Input Files

Mount your OpenAPI specification files:

```bash
docker run --rm -v /path/to/specs:/workspace ghcr.io/developerkunal/openmorph:latest --input /workspace
```

### Configuration

Mount configuration file:

```bash
docker run --rm \
  -v /path/to/specs:/workspace \
  -v /path/to/openmorph.yaml:/config/openmorph.yaml \
  ghcr.io/developerkunal/openmorph:latest \
  --input /workspace \
  --config /config/openmorph.yaml
```

### Output Directory

Mount output directory for results:

```bash
docker run --rm \
  -v /path/to/specs:/workspace \
  -v /path/to/output:/output \
  ghcr.io/developerkunal/openmorph:latest \
  --input /workspace \
  --output /output/transformed.yaml
```

## Environment Variables

```bash
# Enable debug output
docker run --rm -e OPENMORPH_DEBUG=1 -v $(pwd):/workspace ghcr.io/developerkunal/openmorph:latest

# Set working directory
docker run --rm -e WORKDIR=/custom/path -v $(pwd):/custom/path ghcr.io/developerkunal/openmorph:latest
```

## Security Best Practices

### 1. Use Specific Tags

```bash
# Instead of :latest
docker pull ghcr.io/developerkunal/openmorph:v1.2.3
```

### 2. Read-only Mounts

```bash
docker run --rm -v $(pwd):/workspace:ro ghcr.io/developerkunal/openmorph:latest
```

### 3. Non-root User

The production images run as non-root user automatically.

### 4. Resource Limits

```bash
docker run --rm --memory=512m --cpus=1 -v $(pwd):/workspace ghcr.io/developerkunal/openmorph:latest
```

### 5. Distroless for Maximum Security

```bash
docker build -f Dockerfile.distroless -t openmorph:secure .
```

## Troubleshooting

### Debug Mode

```bash
# Enable debug output
docker run --rm -e OPENMORPH_DEBUG=1 -v $(pwd):/workspace ghcr.io/developerkunal/openmorph:latest

# Use development image for debugging
docker run --rm -it -v $(pwd):/workspace openmorph:dev bash
```

### File Permissions

```bash
# If you encounter permission issues
docker run --rm --user $(id -u):$(id -g) -v $(pwd):/workspace ghcr.io/developerkunal/openmorph:latest
```

### Check Version

```bash
docker run --rm ghcr.io/developerkunal/openmorph:latest --version
```

## Performance Tips

1. **Use .dockerignore** - Reduces build context size
2. **Multi-stage builds** - Smaller final images
3. **Layer caching** - Order instructions for optimal caching
4. **Resource limits** - Prevent resource exhaustion in CI/CD

## Examples

### Basic Transformation

```bash
docker run --rm -v $(pwd):/workspace ghcr.io/developerkunal/openmorph:latest \
  --input /workspace \
  --inline-map "x-custom:x-vendor" \
  --dry-run
```

### With Configuration File

```bash
docker run --rm -v $(pwd):/workspace ghcr.io/developerkunal/openmorph:latest \
  --input /workspace \
  --config /workspace/openmorph.yaml \
  --output /workspace/output.yaml
```

### Batch Processing

```bash
# Process multiple directories
for dir in spec1 spec2 spec3; do
  docker run --rm -v $(pwd)/$dir:/workspace ghcr.io/developerkunal/openmorph:latest \
    --input /workspace \
    --output /workspace/transformed.yaml
done
```
