#!/bin/bash

# Generate SVG from all .mmd files under the assets folder
# Output SVG is placed alongside the source .mmd file
# Usage: ./generate-diagrams.sh docs/assets/diagrams

ASSETS_DIR="${1:-assets}"

if ! command -v mmdc &> /dev/null; then
  echo "Error: mmdc not found. Run: npm install -g @mermaid-js/mermaid-cli"
  exit 1
fi

find "$ASSETS_DIR" -name "*.mmd" | while read -r mmd_file; do
  svg_file="${mmd_file%.mmd}.svg"
  png_file="${mmd_file%.mmd}.png"
  echo "Generating: $svg_file"
  mmdc -i "$mmd_file" -o "$svg_file"
  echo "Generating: $png_file"
  mmdc -i "$mmd_file" -o "$png_file"
done

echo "Done."
