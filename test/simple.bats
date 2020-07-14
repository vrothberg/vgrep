#!/usr/bin/env bats -t

@test "Simple search" {
	run ./build/vgrep f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "Index" ]]
	[[ ${lines[0]} =~ "File" ]]
	[[ ${lines[0]} =~ "Line" ]]
	[[ ${lines[0]} =~ "Content" ]]
	[[ ${lines[1]} =~ "0" ]]
	[[ ${lines[2]} =~ "1" ]]
	[[ ${lines[101]} =~ "100" ]]
}

@test "Simple search and --no-git" {
	run ./build/vgrep --no-git f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

@test "Simple search and --no-ripgrep" {
	run ./build/vgrep --no-ripgrep f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

@test "Simple search and --no-git --no-ripgrep" {
	run ./build/vgrep --no-git --no-ripgrep f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

@test "Simple search and --no-header" {
	run ./build/vgrep --no-header f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

@test "Simple search and --no-less" {
	run ./build/vgrep --no-less f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

# Check that all grep tools are used when expected

@test "Search with ripgrep" {
	run ./build/vgrep -d some_pattern 2>&1
	[[ ${lines[@]} =~ "rg -0" ]]
}

@test "Search with git grep" {
	run ./build/vgrep -d --no-ripgrep some_pattern 2>&1
	[[ ${lines[@]} =~ "git -c color.grep.match=red bold grep" ]]
}

@test "Search with classic grep" {
	run ./build/vgrep -d --no-ripgrep --no-git some_pattern 2>&1
	[[ ${lines[@]} =~ "grep -ZHInr" ]]
}

@test "Fallback to classic grep with --no-ripgrep and outside of a git repo" {
	tmp=$(mktemp -d)
	vgrep_repo="$PWD"
	pushd $tmp
	run "$vgrep_repo"/build/vgrep -d --no-ripgrep some_pattern 2>&1
	popd
	rmdir $tmp
	[[ ${lines[@]} =~ "grep -ZHInr" ]]
}

# Other checks

@test "Fail gracefully with error message when unable to parse output" {
	run ./build/vgrep -d -C5 peanut
	[ "$status" -eq 1 ]
	[[ ${lines[@]} =~ "failed to parse results, did you use an option that modifies the output?" ]]
	[[ ${lines[@]} =~ "level=debug msg=\"failed to parse:" ]]
}
