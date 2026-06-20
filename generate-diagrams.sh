#!/bin/bash

# Generate SVG and PNG from all .mmd files under the assets folder.
# Output files are placed alongside the source .mmd file.
#
# Usage:
#   ./generate-diagrams.sh                    # process all diagrams
#   ./generate-diagrams.sh docs/assets/diagrams/containers  # single diagram
#
# Requirements:
#   npm install -g @mermaid-js/mermaid-cli

ASSETS_DIR="${1:-docs/assets/diagrams}"
CONFIG_FILE="docs/assets/mmdc-config.json"
PNG_SCALE=3   # 3× pixel density → ~288 dpi; increase to 4 for print quality

if ! command -v mmdc &> /dev/null; then
  echo "Error: mmdc not found. Run: npm install -g @mermaid-js/mermaid-cli"
  exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
  echo "Warning: config file '$CONFIG_FILE' not found — using mmdc defaults"
  CONFIG_ARGS=""
else
  CONFIG_ARGS="--configFile $CONFIG_FILE"
fi

find "$ASSETS_DIR" -name "*.mmd" | while read -r mmd_file; do
  svg_file="${mmd_file%.mmd}.svg"
  png_file="${mmd_file%.mmd}.png"

  echo "Generating: $svg_file"
  mmdc -i "$mmd_file" -o "$svg_file" --backgroundColor white $CONFIG_ARGS

  echo "Generating: $png_file"
  mmdc -i "$mmd_file" -o "$png_file" --backgroundColor white --scale $PNG_SCALE $CONFIG_ARGS
done

echo "Done."
