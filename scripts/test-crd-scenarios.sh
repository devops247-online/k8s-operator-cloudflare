#!/bin/bash

set -e

CHART_PATH="charts/cloudflare-dns-operator"
CHART_NAME="cloudflare-dns-operator"

echo "Testing CRD conditional installation scenarios..."

# Test 1: Install with CRDs enabled
echo "Test 1: Installing with CRDs enabled"
helm template test-crd-install $CHART_PATH \
  --values $CHART_PATH/ci/crd-install-test.yaml \
  --show-only templates/crds/cloudflarerecords.yaml > /tmp/crd-install-output.yaml

if grep -q "kind: CustomResourceDefinition" /tmp/crd-install-output.yaml; then
  echo "✅ CRD found in output when crds.install=true"
else
  echo "❌ CRD not found in output when crds.install=true"
  exit 1
fi

# Test 2: Install with CRDs disabled
echo "Test 2: Installing with CRDs disabled"
helm template test-crd-no-install $CHART_PATH \
  --values $CHART_PATH/ci/crd-no-install-test.yaml \
  --show-only templates/crds/cloudflarerecords.yaml > /tmp/crd-no-install-output.yaml || true

if [ ! -s /tmp/crd-no-install-output.yaml ]; then
  echo "✅ No CRD output when crds.install=false"
else
  echo "❌ CRD found in output when crds.install=false"
  exit 1
fi

# Test 3: Install with CRDs and keep policy
echo "Test 3: Installing with CRDs and keep policy"
helm template test-crd-keep $CHART_PATH \
  --values $CHART_PATH/ci/crd-keep-test.yaml \
  --show-only templates/crds/cloudflarerecords.yaml > /tmp/crd-keep-output.yaml

if grep -q "helm.sh/resource-policy.*keep" /tmp/crd-keep-output.yaml; then
  echo "✅ CRD has keep annotation when crds.keep=true"
else
  echo "❌ CRD missing keep annotation when crds.keep=true"
  exit 1
fi

# Test 4: Full chart template test with CRDs enabled
echo "Test 4: Full chart template with CRDs enabled"
helm template test-full $CHART_PATH \
  --values $CHART_PATH/ci/crd-install-test.yaml > /tmp/full-template-output.yaml

if grep -q "kind: CustomResourceDefinition" /tmp/full-template-output.yaml; then
  echo "✅ CRD found in full template output"
else
  echo "❌ CRD not found in full template output"
  exit 1
fi

# Test 5: Full chart template test with CRDs disabled
echo "Test 5: Full chart template with CRDs disabled"
helm template test-full-no-crd $CHART_PATH \
  --values $CHART_PATH/ci/crd-no-install-test.yaml > /tmp/full-template-no-crd-output.yaml

if ! grep -q "kind: CustomResourceDefinition" /tmp/full-template-no-crd-output.yaml; then
  echo "✅ No CRD found in full template output when disabled"
else
  echo "❌ CRD found in full template output when disabled"
  exit 1
fi

# Test 6: Lint tests
echo "Test 6: Helm lint with various configurations"
helm lint $CHART_PATH --values $CHART_PATH/ci/crd-install-test.yaml
helm lint $CHART_PATH --values $CHART_PATH/ci/crd-no-install-test.yaml
helm lint $CHART_PATH --values $CHART_PATH/ci/crd-keep-test.yaml

echo "All CRD conditional installation tests passed! ✅"

# Cleanup
rm -f /tmp/crd-*-output.yaml /tmp/full-template-*-output.yaml
