#!/usr/bin/env bats -t

load helpers

# Test that we support passing directories or files to search to the grep
# tools. Check that we do not panic, that we find the good match, and that only
# relevant paths are searched.

# Passing one subdirectory to search

SUBDIR=test/search_files

@test "Search one subdir" {
	run_vgrep foo $SUBDIR
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "bar baz" ]]
	# Check it does not match _this_ file
	[[ ! ${lines[@]} =~ "SUBDIR" ]]
}

@test "Search one subdir and --no-git" {
	run_vgrep --no-git foo $SUBDIR
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "bar baz" ]]
	[[ ! ${lines[@]} =~ "SUBDIR" ]]
}

@test "Search one subdir and --no-ripgrep" {
	run_vgrep --no-ripgrep foo $SUBDIR
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "bar baz" ]]
	[[ ! ${lines[@]} =~ "SUBDIR" ]]
}

@test "Search one subdir and --no-git --no-ripgrep" {
	run_vgrep --no-git --no-ripgrep foo $SUBDIR
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "bar baz" ]]
	[[ ! ${lines[@]} =~ "SUBDIR" ]]
}

# Passing a single file to search
#
# There is a difference with searching a directory, because some tools
# (grep/ripgrep) do not print the filename by default in that case. This would
# break output parsing if vgrep did not force them to print the filename
# unconditionally.

FILE=$SUBDIR/foobar.txt

@test "Search single file" {
	run_vgrep foo $FILE
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "bar baz" ]]
	[[ ! ${lines[@]} =~ "SUBDIR" ]]
}

@test "Search single file and --no-git" {
	run_vgrep --no-git foo $FILE
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "bar baz" ]]
	[[ ! ${lines[@]} =~ "SUBDIR" ]]
}

@test "Search single file and --no-ripgrep" {
	run_vgrep --no-ripgrep foo $FILE
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "bar baz" ]]
	[[ ! ${lines[@]} =~ "SUBDIR" ]]
}

@test "Search single file and --no-git --no-ripgrep" {
	run_vgrep --no-git --no-ripgrep foo $FILE
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "bar baz" ]]
	[[ ! ${lines[@]} =~ "SUBDIR" ]]
}
