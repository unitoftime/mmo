#!/bin/bash

# TODO - Convert to go file? generator.go? https://stackoverflow.com/questions/55598931/go-generate-multiline-command

set -x # Activate debugging

echo "Exporting Aseprite Files to ase/images"
mkdir -p ase/images
rm -f ase/images/*

# Exporting with using tags and trimming
# filenames=(menu_ui particles)
# for file in ${filenames[@]}
# do
#     aseprite -b ${file}.ase --trim --save-as export/${file}_{tag}{tagframe0}.png
#     mogrify -trim export/${file}_*.png
# done

# Exporting with using tags
filenames=(man hat-top ui)
for file in ${filenames[@]}
do
#    aseprite -b ${file}.ase --save-as images/${file}_{tag}{tagframe0}.png
    aseprite -b ase/${file}.ase --format json-array --list-tags --data assets/${file}.json --save-as "ase/images/${file}_{frame}.png"
done

# Exporting without using tags
filenames=(dirt grass water)
for file in ${filenames[@]}
do
    aseprite -b ase/${file}.ase --save-as ase/images/${file}{frame}.png
done
