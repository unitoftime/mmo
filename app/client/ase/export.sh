#!/bin/bash

# TODO - Convert to go file? generator.go? https://stackoverflow.com/questions/55598931/go-generate-multiline-command

set -x # Activate debugging

echo "Exporting Aseprite Files to ase/images"
mkdir -p ase/images

# Exporting with using tags and trimming
# filenames=(menu_ui particles)
# for file in ${filenames[@]}
# do
#     aseprite -b ${file}.ase --trim --save-as export/${file}_{tag}{tagframe0}.png
#     mogrify -trim export/${file}_*.png
# done

# Exporting Animated Objects
filenames=(man hat-top hat-bycocket hat-mohawk hat-nightcap)
for file in ${filenames[@]}
do
#    aseprite -b ${file}.ase --save-as images/${file}_{tag}{tagframe0}.png
    aseprite -b ase/${file}.ase --format json-array --ignore-layer=mount --list-tags --data assets/${file}.json --save-as "ase/images/${file}_{frame}.png"
    aseprite -b ase/${file}.ase --format json-array --layer=mount --list-tags --data assets/${file}.json --save-as "ase/mount/${file}_{frame}.png"
done

# Exporting static objects defined by tags
filenames=(ui)
for file in ${filenames[@]}
do
    aseprite -b ase/${file}.ase --format json-array --list-tags --data assets/${file}.json --save-as "ase/images/${file}_{tag}{tagframe}.png"
done

# Exporting without using tags
filenames=(dirt grass water concrete wall)
for file in ${filenames[@]}
do
    aseprite -b ase/${file}.ase --save-as ase/images/${file}{frame}.png
done

# Pack all images into a spritesheet
packer --input ase/images --stats --output assets/spritesheet

#go run github.com/unitoftime/packer/cmd/packer --input ase/mount --mountpoints --stats --output assets/mountpoints
packer --input ase/mount --mountpoints --stats --output assets/mountpoints

# Remove generated images
rm -f ase/images/*
rm -f ase/mount/*
