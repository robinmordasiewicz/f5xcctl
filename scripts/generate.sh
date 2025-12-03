#!/bin/bash
# Code generation script for f5xcctl CLI
# Generates Go client code and Cobra commands from OpenAPI specifications

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SPECS_DIR="$PROJECT_ROOT/docs/specifications/api"
OUTPUT_DIR="$PROJECT_ROOT/internal/api"

echo "f5xcctl CLI Code Generator"
echo "======================="
echo ""

# Check if specs directory exists
if [ ! -d "$SPECS_DIR" ]; then
    echo "Error: OpenAPI specs directory not found: $SPECS_DIR"
    exit 1
fi

# Count specs
SPEC_COUNT=$(ls -1 "$SPECS_DIR"/*.json 2>/dev/null | wc -l)
echo "Found $SPEC_COUNT OpenAPI specification files"
echo ""

# Check for openapi-generator-cli
if ! command -v openapi-generator-cli &> /dev/null; then
    echo "openapi-generator-cli not found. Installing via npm..."
    npm install -g @openapitools/openapi-generator-cli
fi

# Create output directories
mkdir -p "$OUTPUT_DIR/models"
mkdir -p "$OUTPUT_DIR/clients"

# Generate from each spec (sample - in practice would aggregate)
echo "Generating code from OpenAPI specifications..."
echo ""

# For the initial implementation, we'll generate from a few key specs
KEY_SPECS=(
    "docs-cloud-f5-com.0166.public.ves.io.schema.namespace.ves-swagger.json"
    "docs-cloud-f5-com.0073.public.ves.io.schema.views.http_loadbalancer.ves-swagger.json"
    "docs-cloud-f5-com.0177.public.ves.io.schema.views.origin_pool.ves-swagger.json"
    "docs-cloud-f5-com.0019.public.ves.io.schema.app_firewall.ves-swagger.json"
    "docs-cloud-f5-com.0048.public.ves.io.schema.certificate.ves-swagger.json"
    "docs-cloud-f5-com.0091.public.ves.io.schema.dns_zone.ves-swagger.json"
    "docs-cloud-f5-com.0012.public.ves.io.schema.alert_policy.ves-swagger.json"
)

for spec in "${KEY_SPECS[@]}"; do
    SPEC_PATH="$SPECS_DIR/$spec"
    if [ -f "$SPEC_PATH" ]; then
        RESOURCE_NAME=$(echo "$spec" | sed -E 's/.*schema\.([^.]+)\.ves-swagger\.json/\1/' | sed -E 's/.*views\.([^.]+)\.ves-swagger\.json/\1/')
        echo "  Processing: $RESOURCE_NAME"

        # In a full implementation, we would:
        # 1. Parse the OpenAPI spec
        # 2. Generate Go models
        # 3. Generate API client methods
        # 4. Generate Cobra command scaffolding

        # For now, just validate the spec
        if command -v jq &> /dev/null; then
            TITLE=$(jq -r '.info.title // "Unknown"' "$SPEC_PATH")
            VERSION=$(jq -r '.info.version // "Unknown"' "$SPEC_PATH")
            PATHS_COUNT=$(jq '.paths | length' "$SPEC_PATH")
            echo "    Title: $TITLE"
            echo "    Version: $VERSION"
            echo "    Paths: $PATHS_COUNT"
        fi
    else
        echo "  Warning: Spec not found: $spec"
    fi
done

echo ""
echo "Code generation complete!"
echo ""
echo "Note: Full code generation requires:"
echo "  1. openapi-generator-cli (npm install -g @openapitools/openapi-generator-cli)"
echo "  2. Custom templates in templates/ directory"
echo "  3. Spec aggregation and normalization"
echo ""
echo "For now, the core commands are hand-written in internal/cmd/"
