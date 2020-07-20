#!/bin/bash
set -ex

platform=$1
win=$2

rm -rf tmp
mkdir tmp
mkdir -p dist

mktarball() {
  dir=$1
  if [ "$win" = "" ]; then
    tar cJf dist/$dir.tar.xz -C tmp $dir
  else
    (cd tmp && zip -r ../dist/$dir.zip $dir)
  fi
}

# Create the main tarball of binaries
bin_pkgname=randomx-$platform
mkdir tmp/$bin_pkgname

mv c-api-$platform/* tmp/$bin_pkgname
mktarball $bin_pkgname
