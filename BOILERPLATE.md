# Boilerplate Integration Guide

This project uses the [OpenShift Boilerplate](https://github.com/openshift/boilerplate) system for standardized build, test, and CI/CD workflows.

## What is Boilerplate?

Boilerplate is a system for maintaining consistent tooling and conventions across OpenShift projects. It provides:

- Standardized Makefiles with common targets
- CI/CD integration (Prow)
- Code quality tools (linting, formatting, coverage)
- Container build workflows
- FIPS compliance support
- OLM (Operator Lifecycle Manager) support

## Project Structure

```
ebs-metrics-exporter/
├── boilerplate/
│   ├── update              # Script to update boilerplate
│   ├── update.cfg          # Configuration: which conventions to use
│   ├── generated-includes.mk  # Auto-generated Makefile includes
│   └── openshift/          # Populated by 'make boilerplate-update'
│       └── golang-osd-operator/
├── project.mk              # Project-specific configuration
├── Makefile                # Main Makefile (includes boilerplate)
└── .ci-operator.yaml       # Prow CI configuration
```

## Conventions Used

This project uses the `openshift/golang-osd-operator` convention, which provides:

- **Build targets**: `make go-build`, `make docker-build`
- **Test targets**: `make test`, `make coverage`, `make lint`
- **CI integration**: Automatic Prow CI configuration
- **FIPS support**: Optional FIPS-compliant builds
- **OLM support**: Operator bundle and catalog generation

## Initial Setup

### 1. Initialize Boilerplate

On first checkout, run:

```bash
make boilerplate-update
```

This will:
- Clone the boilerplate repository
- Copy convention files to `boilerplate/openshift/golang-osd-operator/`
- Generate `boilerplate/generated-includes.mk`
- Set up necessary tooling

### 2. Verify Installation

```bash
# Should show all available targets
make help

# Test that boilerplate is working
make validate
```

## Common Targets

### Building

```bash
# Build the operator binary
make go-build

# Build operator container image
make docker-build-operator

# Build exporter DaemonSet image
make docker-build-exporter

# Build both images
make docker-build
```

### Testing

```bash
# Run unit tests
make test

# Run linting
make lint

# Generate coverage report
make coverage

# Run all validation
make validate
```

### Deployment

```bash
# Deploy operator
make deploy-operator

# Deploy exporter DaemonSet
make deploy-exporter

# Deploy everything
make deploy-all

# Undeploy everything
make undeploy-all
```

### Maintenance

```bash
# Update boilerplate to latest version
make boilerplate-update

# This should be done periodically to get:
# - New features
# - Bug fixes
# - Security updates
# - Updated CI configurations
```

## Configuration Files

### `boilerplate/update.cfg`

Specifies which boilerplate conventions to use:

```
# Use standards for Go-based OSD operators
openshift/golang-osd-operator
```

### `project.mk`

Project-specific variables and overrides:

```makefile
OPERATOR_NAME := ebs-metrics-exporter
OPERATOR_NAMESPACE := openshift-sre-ebs-metrics
FIPS_ENABLED := true
```

### `.ci-operator.yaml`

Prow CI configuration:
- Build root image (Go version)
- Test steps (unit, lint, coverage)
- Container image builds
- Resource requirements

## FIPS Support

This project has FIPS mode enabled by default:

```makefile
FIPS_ENABLED := true
```

When enabled:
- Binary is built with FIPS-compliant crypto
- Special build tags are applied
- `fips.go` is auto-generated (if using `make generate`)

To disable:
```makefile
FIPS_ENABLED := false
```

## CI/CD Integration

### Prow CI

The `.ci-operator.yaml` file configures OpenShift Prow CI to:

1. **Build**: Compile the operator and exporter
2. **Test**: Run unit tests
3. **Lint**: Check code quality
4. **Coverage**: Generate coverage reports
5. **Build Images**: Create container images

### Local CI Simulation

Boilerplate provides container-based testing:

```bash
# Run tests in a container (simulates CI)
make container-test

# Run lint in a container
make container-lint
```

## Updating Boilerplate

### When to Update

- **Regularly**: Monthly or quarterly
- **Before releases**: To get latest tooling
- **When notified**: If boilerplate team announces important updates

### How to Update

```bash
# 1. Update boilerplate
make boilerplate-update

# 2. Review changes
git diff

# 3. Test changes
make validate
make test

# 4. Commit if everything works
git add boilerplate/
git commit -m "Update boilerplate"
```

### Handling Conflicts

If boilerplate updates conflict with local changes:

1. **Generated files**: Always accept boilerplate version
   - `boilerplate/generated-includes.mk`
   - Auto-generated code

2. **Project files**: Review carefully
   - `project.mk` - May need manual merge
   - Custom targets in `Makefile`

## Advanced Usage

### Custom Makefile Targets

Add project-specific targets to `Makefile` (not `project.mk`):

```makefile
# Custom target for this project
.PHONY: deploy-test-cluster
deploy-test-cluster:
	@echo "Deploying to test cluster..."
	oc apply -f test/manifests/
```

### Multi-Image Builds

This project builds two images (operator + exporter). Configure in `project.mk`:

```makefile
ADDITIONAL_IMAGE_SPECS := ebs-metrics-exporter-daemonset=Dockerfile
```

Boilerplate will automatically handle both images.

### Customizing CI

Edit `.ci-operator.yaml` to:
- Change Go version
- Add integration tests
- Modify resource limits
- Add deployment tests

## Troubleshooting

### Boilerplate update fails

```bash
# Clear cache and retry
rm -rf /tmp/boilerplate
make boilerplate-update
```

### Make targets not found

```bash
# Ensure generated-includes.mk exists
ls -la boilerplate/generated-includes.mk

# Regenerate if missing
make boilerplate-update
```

### FIPS build errors

```bash
# Disable FIPS temporarily
make go-build FIPS_ENABLED=false

# Or edit project.mk:
FIPS_ENABLED := false
```

### Container build fails

```bash
# Check Docker/Podman is running
docker ps

# Verify Dockerfile syntax
docker build -f Dockerfile.operator .
```

## Best Practices

1. **Regular updates**: Run `make boilerplate-update` regularly
2. **Don't edit generated files**: They will be overwritten
3. **Use project.mk**: For project-specific configuration
4. **Test before committing**: Run `make validate test` after updates
5. **Version control**: Commit boilerplate changes separately from code changes

## Resources

- [OpenShift Boilerplate Repository](https://github.com/openshift/boilerplate)
- [Golang OSD Operator Convention](https://github.com/openshift/boilerplate/tree/master/boilerplate/openshift/golang-osd-operator)
- [OSD Metrics Exporter](https://github.com/openshift/osd-metrics-exporter) (reference implementation)

## Getting Help

- Check boilerplate README: https://github.com/openshift/boilerplate
- Review convention docs: `boilerplate/openshift/golang-osd-operator/README.md` (after update)
- Ask in #forum-osd-sre (internal)
