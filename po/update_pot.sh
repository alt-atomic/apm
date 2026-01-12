#!/bin/sh
# Should run from project root dir

sh po/update_potfiles.sh

cat ./po/POTFILES | xargs xgettext --language=C --keyword=T_ --keyword=TN_:1,2 --keyword=TD_:2 --keyword=TC_:1c,2 -o po/apm.pot --from-code=UTF-8 --add-comments --package-name=apm

# TODO: Add autogeneration
cat << EOF >> po/apm.pot

#: data/apt-bridge/update-apm.lua:6
msgid "Updating apm database"
msgstr ""
EOF
