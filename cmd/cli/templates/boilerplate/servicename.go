package boilerplate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// serviceNameFiles lists non-Go files where the literal service name
// "go-boilerplate" appears and must be replaced with the new name.
//
// Go files are NOT listed here because they use the full module path
// (github.com/jrmarcello/go-boilerplate) which is handled separately
// by the module rewriter.
var serviceNameFiles = []string{
	// Build
	"Makefile",

	// Docker
	"docker/Dockerfile",
	"docker/docker-compose.yml",
	"docker/observability/docker-compose.yml",

	// Kubernetes base
	"deploy/base/deployment.yaml",
	"deploy/base/hpa.yaml",
	"deploy/base/ingress.yaml",
	"deploy/base/migration-job.yaml",
	"deploy/base/networkpolicy.yaml",
	"deploy/base/pdb.yaml",
	"deploy/base/service.yaml",
	"deploy/base/serviceaccount.yaml",

	// Kubernetes overlays — develop
	"deploy/overlays/develop/configmap.yaml",
	"deploy/overlays/develop/deployment-patch.yaml",
	"deploy/overlays/develop/hpa-patch.yaml",
	"deploy/overlays/develop/kustomization.yaml",
	"deploy/overlays/develop/secret.yaml",

	// Kubernetes overlays — homologacao
	"deploy/overlays/homologacao/configmap.yaml",
	"deploy/overlays/homologacao/deployment-patch.yaml",
	"deploy/overlays/homologacao/hpa-patch.yaml",
	"deploy/overlays/homologacao/ingress-host-patch.yaml",
	"deploy/overlays/homologacao/kustomization.yaml",
	"deploy/overlays/homologacao/secret.yaml",

	// Kubernetes overlays — producao
	"deploy/overlays/producao/configmap.yaml",
	"deploy/overlays/producao/deployment-patch.yaml",
	"deploy/overlays/producao/hpa-patch.yaml",
	"deploy/overlays/producao/ingress-host-patch.yaml",
	"deploy/overlays/producao/kustomization.yaml",
	"deploy/overlays/producao/secret.yaml",

	// Lint config
	".golangci.yml",

	// Documentation
	"README.md",

	// Swagger (generated, but included in snapshot)
	"docs/swagger.yaml",
	"docs/swagger.json",
}

const defaultServiceName = "go-boilerplate"

// ReplaceServiceName replaces all occurrences of "go-boilerplate" with
// newName in the known config and deploy files within projectDir.
// Files that were removed by feature toggles are silently skipped.
func ReplaceServiceName(projectDir, newName string) error {
	for _, relPath := range serviceNameFiles {
		absPath := filepath.Join(projectDir, relPath)

		content, readErr := os.ReadFile(absPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return fmt.Errorf("reading %s: %w", relPath, readErr)
		}

		oldContent := string(content)
		newContent := strings.ReplaceAll(oldContent, defaultServiceName, newName)

		if oldContent == newContent {
			continue
		}

		info, statErr := os.Stat(absPath)
		if statErr != nil {
			return fmt.Errorf("stat %s: %w", relPath, statErr)
		}

		writeErr := os.WriteFile(absPath, []byte(newContent), info.Mode()) //nolint:gosec // CLI tool writes to user-specified project directory
		if writeErr != nil {
			return fmt.Errorf("writing %s: %w", relPath, writeErr)
		}
	}
	return nil
}
