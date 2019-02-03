#!/bin/bash
set -e

if [[ -z $(git status --porcelain) ]]
then
	echo "tree is clean"
else
	echo "tree is dirty, please commit all changes and sync the vendor.conf"
	exit
fi
