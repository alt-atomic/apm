#!/bin/sh
# Should run from project root dir

for lang in $(cat po/LINGUAS); do
    msgmerge -U --backup=off po/$lang.po po/apm.pot
done
