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
