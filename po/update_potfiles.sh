#!/bin/sh
# Should run from project root dir

touch ./po/unsort-POTFILES

find ./ -iname "*.go" -type f -exec grep -lrE 'T_\(|TN_\(|TD_\(|TC_\(' {} + | while read file; do echo "${file#./}" >> ./po/unsort-POTFILES; done

cat ./po/unsort-POTFILES | sort | uniq > ./po/POTFILES

rm ./po/unsort-POTFILES
