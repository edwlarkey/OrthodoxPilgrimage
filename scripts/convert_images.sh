#!/bin/bash

# Required: ImageMagick (magick or convert/mogrify)
# Usage: ./optimize-images.sh ./path/to/raw/images

INPUT_DIR=$1
OUTPUT_DIR="${INPUT_DIR}/optimized"

if [ -z "$INPUT_DIR" ]; then
    echo "Usage: $0 <directory>"
    exit 1
fi

mkdir -p "$OUTPUT_DIR"

echo "Processing images in $INPUT_DIR..."

# 1. Loop through jpg, jpeg, and png files
for img in "$INPUT_DIR"/*.{jpg,jpeg,png,JPG,JPEG,PNG}; do
    # Skip if no files match
    [ -e "$img" ] || continue

    filename=$(basename -- "$img")
    extension="${filename##*.}"
    filename_no_ext="${filename%.*}"

    echo "Optimizing: $filename"

    # 2. Conversion settings:
    # -auto-orient: READS the orientation metadata and ROTATES the actual pixels
    # -resize: Max width 800px, height calculated automatically
    # -quality: 80 (standard balance for WebP)
    # -strip: Removes EXIF metadata and color profiles
    # -define webp:method=6: Use highest compression effort (takes longer, smaller file)
    magick "$img" \
        -auto-orient \
        -resize "800x>" \
        -strip \
        -quality 80 \
        -define webp:method=6 \
        "$OUTPUT_DIR/${filename_no_ext}.webp"
done

echo "Done! Optimized images are in: $OUTPUT_DIR"


