#!/usr/bin/env bats -t

load helpers

# Create a write-only file which causes the *grep implementations to fail. In
# that case, vgrep should still parse the stdout of *grep and return the error
# reported on stdout along with the exit code.

WONLY_FILE=test/search_files/wonly.txt

function setup() {
	touch $WONLY_FILE
	chmod 200 $WONLY_FILE
}

function teardown() {
	rm $WONLY_FILE
}

@test "Search with permission error" {
	run_vgrep --no-header foo test/search_files
	[ "$status" -eq 2 ]
	[[ ${lines[0]} =~ "test/search_files/wonly.txt: Permission denied (os error 13)" ]]
	[[ ${lines[1]} =~ "bar baz" ]]
}

@test "Search with permission error (--no-ripgrep)" {
        # Since the file isn't under version control, git will not try to read it
	run_vgrep --no-header --no-ripgrep foo test/search_files
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "bar baz" ]]
}

@test "Search with permission error (--no-git --no-ripgrep)" {
	run_vgrep --no-header --no-git --no-ripgrep foo test/search_files
	[ "$status" -eq 2 ]
	[[ ${lines[0]} =~ "test/search_files/wonly.txt: Permission denied" ]]
	[[ ${lines[1]} =~ "bar baz" ]]
}
