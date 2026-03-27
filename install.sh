#!/bin/bash
set -e

SYDNEY_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Building Sydney compiler..."
cd "$SYDNEY_DIR"
go build -o sydney

echo "Building Sydney LSP..."
go build -o bin/sydney-lsp ./cmd/lsp/

echo "Building Sydney runtime..."
cd "$SYDNEY_DIR/sydney_rt"
cargo build --release

echo "Configuring fish shell..."
fish -c "set -Ux SYDNEY_PATH $SYDNEY_DIR"
fish -c "fish_add_path $SYDNEY_DIR"
fish -c "fish_add_path $SYDNEY_DIR/bin"

echo "Done."
echo "  sydney:     $SYDNEY_DIR/sydney"
echo "  sydney-lsp: $SYDNEY_DIR/bin/sydney-lsp"
echo "  SYDNEY_PATH=$SYDNEY_DIR"
