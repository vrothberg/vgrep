#!/usr/bin/env bats -t

load helpers

@test "Simple search" {
	run_vgrep f
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
	run_vgrep --no-git f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

@test "Simple search and --no-ripgrep" {
	run_vgrep --no-ripgrep f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

@test "Simple search and --no-git --no-ripgrep" {
	run_vgrep --no-git --no-ripgrep f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

@test "Simple search and --no-header" {
	run_vgrep --no-header f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

@test "Simple search and --no-less" {
	run_vgrep --no-less f
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "0" ]]
}

# Check that all grep tools are used when expected

@test "Search with ripgrep" {
	run_vgrep -d some_pattern 2>&1
	[[ ${lines[@]} =~ "rg -0" ]]
}

@test "Search with git grep" {
	run_vgrep -d --no-ripgrep some_pattern 2>&1
	[[ ${lines[@]} =~ "git -c color.grep.match=red bold grep" ]]
}

@test "Search with classic grep" {
	run_vgrep -d --no-ripgrep --no-git some_pattern 2>&1
	[[ ${lines[@]} =~ "grep -ZHInr" ]]
}

@test "Fallback to classic grep with --no-ripgrep and outside of a git repo" {
	tmp=$(mktemp -d)
	pushd $tmp
	run_vgrep -d --no-ripgrep some_pattern 2>&1
	popd
	rmdir $tmp
	echo $VGREP
	[[ ${lines[@]} =~ "grep -ZHInr" ]]
}

# Other checks

@test "Search with -C5 for context lines" {
	run_vgrep -d -C5 peanut
	[ "$status" -eq 0 ]
	echo "${lines[@]}"
	[[ ${lines[@]} =~ "level=debug msg=\"skipping line \\\"--\\\" (parse error: expected 2 but split into 1 items ([0]))\"" ]]
}

@test "Exit with 1 when a search has no matches" {
	run_vgrep "^cashew$"
	[ "$status" -eq 1 ]
	[[ ${#lines[*]} -eq 0 ]]
}

@test "Exit with 1 when operating on empty results" {
	run_vgrep "^cashew$" > /dev/null
	run_vgrep
	[ "$status" -eq 1 ]
	[[ ${#lines[*]} -eq 0 ]]
}
