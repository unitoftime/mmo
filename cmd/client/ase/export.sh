#!/bin/bash

set -x # Activate debugging

echo "Exporting Aseprite Files"

# Exporting with using tags and trimming
# filenames=(menu_ui particles)
# for file in ${filenames[@]}
# do
#     aseprite -b ${file}.ase --trim --save-as export/${file}_{tag}{tagframe0}.png
#     mogrify -trim export/${file}_*.png
# done

# Exporting with using tags
filenames=(man hat-top)
for file in ${filenames[@]}
do
#    aseprite -b ${file}.ase --save-as export/${file}_{tag}{tagframe0}.png
    aseprite -b ${file}.ase --format json-array --list-tags --data export/${file}.json --save-as "export/${file}_{frame}.png"
done

# Exporting without using tags
# filenames=()
# for file in ${filenames[@]}
# do
#     aseprite -b ${file}.ase --save-as export/${file}{frame1}.png
# done

cp ./export/*.png ../images/
