#!/bin/bash
# Build all NWDAF plugins
# Usage: ./scripts/build_plugins.sh [collectors|analytics|all]

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo -e "${BLUE}=== NWDAF Plugin Builder ===${NC}"
echo "Project root: $PROJECT_ROOT"
echo ""

# Determine which Go binary to use
# 1. /usr/local/go/bin/go, 2. system go
GO_BIN=""
if [ -x "/usr/local/go/bin/go" ]; then
    GO_BIN="/usr/local/go/bin/go"
    echo -e "${GREEN}Using Go from: $GO_BIN${NC}"
elif command -v go &> /dev/null; then
    GO_BIN="go"
    echo -e "${YELLOW}Using system Go${NC}"
else
    echo -e "${RED}ERROR: Go is not installed or not in PATH${NC}"
    echo ""
    echo "Please install Go 1.20 or later:"
    echo "  - Ubuntu/Debian: sudo apt install golang-go"
    echo "  - Fedora/RHEL:   sudo dnf install golang"
    echo "  - macOS:         brew install go"
    echo "  - Download:      https://go.dev/dl/"
    echo ""
    exit 1
fi

# Check Go version
GO_VERSION=$("$GO_BIN" version 2>/dev/null | awk '{print $3}' | sed 's/go//')
if [ -n "$GO_VERSION" ]; then
    echo -e "Go version: ${GREEN}${GO_VERSION}${NC}"
else
    echo -e "${YELLOW}Warning: Could not determine Go version${NC}"
fi
echo ""

# Function to build plugins in a directory
build_plugins_in_dir() {
    local plugin_dir=$1
    local plugin_type=$2

    echo -e "${YELLOW}Building ${plugin_type} plugins...${NC}"

    if [ ! -d "$plugin_dir" ]; then
        echo -e "${RED}Error: Directory not found: $plugin_dir${NC}"
        return 1
    fi

    # Create build directory
    mkdir -p "$plugin_dir/build"

    # Count plugins
    plugin_count=0
    success_count=0
    fail_count=0

    # Build each .go file
    for plugin_file in "$plugin_dir"/*.go; do
        # Skip if no .go files found
        [ -e "$plugin_file" ] || continue

        plugin_count=$((plugin_count + 1))
        plugin_name=$(basename "${plugin_file%.go}")
        plugin_relative_path="${plugin_file#$PROJECT_ROOT/}"

        echo -n "  Building: ${plugin_name}... "

        # Print the build command with working directory
        echo ""
        echo -e "  ${BLUE}Working directory: $PROJECT_ROOT${NC}"
        echo -e "  ${BLUE}Command: CGO_ENABLED=0 $GO_BIN build -ldflags=\"-s -w\" -o \"${plugin_dir#$PROJECT_ROOT/}/build/${plugin_name}\" \"./${plugin_relative_path}\"${NC}"
        echo -n "  "

        # Build from project root to maintain module context
        # CGO_ENABLED=0 ensures static binaries that work in musl-based containers
        BUILD_OUTPUT=$(cd "$PROJECT_ROOT" && CGO_ENABLED=0 "$GO_BIN" build -ldflags="-s -w" -o "${plugin_dir#$PROJECT_ROOT/}/build/${plugin_name}" "./${plugin_relative_path}" 2>&1)
        BUILD_EXIT_CODE=$?

        if [ $BUILD_EXIT_CODE -eq 0 ]; then
            # Make executable (important for Docker mounting)
            chmod +x "$plugin_dir/build/${plugin_name}"
            echo -e "${GREEN}✓${NC}"
            success_count=$((success_count + 1))
        else
            echo -e "${RED}✗${NC}"
            echo -e "${RED}    Failed to build ${plugin_name}${NC}"
            if [ -n "$BUILD_OUTPUT" ]; then
                # Show first 3 lines of error
                echo "$BUILD_OUTPUT" | head -3 | while IFS= read -r line; do
                    echo -e "${RED}    $line${NC}"
                done
            fi
            fail_count=$((fail_count + 1))
        fi
    done

    echo ""
    echo -e "  Total: $plugin_count | ${GREEN}Success: $success_count${NC} | ${RED}Failed: $fail_count${NC}"
    echo ""

    return 0
}

# Function to list built plugins
list_plugins() {
    local build_dir=$1
    local plugin_type=$2

    echo -e "${BLUE}${plugin_type} plugins:${NC}"

    if [ -d "$build_dir" ] && [ "$(ls -A "$build_dir" 2>/dev/null)" ]; then
        for plugin in "$build_dir"/*; do
            if [ -f "$plugin" ]; then
                plugin_name=$(basename "$plugin")
                plugin_size=$(du -h "$plugin" | cut -f1)
                if [ -x "$plugin" ]; then
                    echo "  ✓ ${plugin_name} (${plugin_size}) [executable]"
                else
                    echo -e "  ${RED}✗ ${plugin_name} (${plugin_size}) [NOT executable - fixing]${NC}"
                    chmod +x "$plugin"
                fi
            fi
        done
    else
        echo "  (none)"
    fi
    echo ""
}

# Parse command line arguments
MODE="${1:-all}"

case "$MODE" in
    collectors)
        build_plugins_in_dir "$PROJECT_ROOT/plugin/collectors" "Collector"
        ;;
    analytics)
        build_plugins_in_dir "$PROJECT_ROOT/plugin/analytics" "Analytics"
        ;;
    all)
        build_plugins_in_dir "$PROJECT_ROOT/plugin/collectors" "Collector"
        build_plugins_in_dir "$PROJECT_ROOT/plugin/analytics" "Analytics"
        ;;
    *)
        echo -e "${RED}Error: Invalid argument${NC}"
        echo "Usage: $0 [collectors|analytics|all]"
        echo ""
        echo "Examples:"
        echo "  $0              # Build all plugins"
        echo "  $0 all          # Build all plugins"
        echo "  $0 collectors   # Build only collector plugins"
        echo "  $0 analytics    # Build only analytics plugins"
        exit 1
        ;;
esac

# Summary
echo -e "${BLUE}=== Build Summary ===${NC}"
list_plugins "$PROJECT_ROOT/plugin/collectors/build" "Collector"
list_plugins "$PROJECT_ROOT/plugin/analytics/build" "Analytics"

echo -e "${GREEN}Done!${NC}"
echo ""
echo "Next steps:"
echo "  1. Start infrastructure: docker-compose -f docker-compose.infra.yml up -d"
echo "  2. Start NWDAF:          docker-compose up -d"
echo ""
