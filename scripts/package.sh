#!/bin/bash

set -e

mkdir -p build/deb/DEBIAN
make install DESTDIR=build/deb
cp control build/deb/DEBIAN
cp config/conffiles build/deb/DEBIAN



dpkg-deb --root-owner-group -b build/deb output.deb
lintian output.deb
