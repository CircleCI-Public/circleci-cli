#!/bin/sh

set -e

# generate markdown docs
go run main.go usage

# setup output dir
mkdir -p out

# pandoc markdown to html into out dir
for f in docs/*.md;  do
    pandoc "$f" -s -o "out/$(basename "${f%.md}".html)";
done

# move index and update links
mv out/circleci.html out/index.html
sed -i -- 's#<a href="circleci.html">#<a href="index.html">#g' out/*.html
