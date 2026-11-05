#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

DIST_DIR="dist"
CHECKSUMS="${DIST_DIR}/checksums.txt"

if [ ! -f "$CHECKSUMS" ]; then
  echo "Checksums file $CHECKSUMS not found. Run task release:package first." >&2
  exit 1
fi

ARM64_SHA=$(grep "darwin_arm64.tar.gz" "$CHECKSUMS" | awk '{print $1}')
AMD64_SHA=$(grep "darwin_amd64.tar.gz" "$CHECKSUMS" | awk '{print $1}')

if [ -z "$ARM64_SHA" ] || [ -z "$AMD64_SHA" ]; then
  echo "Failed to find darwin sha256 in $CHECKSUMS" >&2
  exit 1
fi

FORMULA_FILE="${DIST_DIR}/vanguard.rb"

cat > "$FORMULA_FILE" <<EOF
class Vanguard < Formula
  desc "Interactive vulnerability shield & package update orchestrator"
  homepage "https://jankrloz.github.io/vanguard/"
  version "${VERSION}"
  license "MIT"

  depends_on "meta-package-manager"
  depends_on "osv-scanner"
  depends_on "trivy"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jankrloz/vanguard/releases/download/v${VERSION}/vanguard_${VERSION}_darwin_arm64.tar.gz"
      sha256 "${ARM64_SHA}"
    else
      url "https://github.com/jankrloz/vanguard/releases/download/v${VERSION}/vanguard_${VERSION}_darwin_amd64.tar.gz"
      sha256 "${AMD64_SHA}"
    end
  end

  def install
    bin.install "vanguard"
    bin.install_symlink "vanguard" => "vg"
    bin.install_symlink "vanguard" => "up"
  end

  test do
    assert_match "Vanguard", shell_output("#{bin}/vanguard --help 2>&1", 2)
  end
end
EOF

echo "✓ Generated Homebrew formula at ${FORMULA_FILE}:"
cat "$FORMULA_FILE"

# If HOMEBREW_TAP_TOKEN or GH_TOKEN is provided and has access, try updating homebrew-tap repo
AUTH_TOKEN="${HOMEBREW_TAP_TOKEN:-${GH_TOKEN:-}}"
if [ -n "$AUTH_TOKEN" ]; then
  echo "Attempting to sync formula to jankrloz/homebrew-tap..."
  CURRENT_SHA=$(curl -s -H "Authorization: token ${AUTH_TOKEN}" \
    "https://api.github.com/repos/jankrloz/homebrew-tap/contents/Formula/vanguard.rb" | grep '"sha"' | head -1 | cut -d '"' -f 4 || true)

  if [ -n "$CURRENT_SHA" ]; then
    ENCODED_CONTENT=$(base64 < "$FORMULA_FILE" | tr -d '\n')
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT \
      -H "Authorization: token ${AUTH_TOKEN}" \
      -H "Content-Type: application/json" \
      "https://api.github.com/repos/jankrloz/homebrew-tap/contents/Formula/vanguard.rb" \
      -d "{\"message\":\"release: vanguard v${VERSION}\",\"content\":\"${ENCODED_CONTENT}\",\"sha\":\"${CURRENT_SHA}\"}")
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
      echo "✓ Successfully updated Formula/vanguard.rb in jankrloz/homebrew-tap!"
    else
      echo "Notice: Could not update homebrew-tap via API (HTTP ${HTTP_CODE}). Update manually or grant tap token access."
    fi
  fi
fi
