#!/usr/bin/env bats -t

load helpers

@test "Show print" {
	run_vgrep f
	[ "$status" -eq 0 ]
	run_vgrep -s p 10
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "Index" ]]
	[[ ${lines[0]} =~ "File" ]]
	[[ ${lines[0]} =~ "Line" ]]
	[[ ${lines[0]} =~ "Content" ]]
	[[ ${lines[1]} =~ "10" ]]
}

@test "Show context with selectors" {
	run_vgrep f
	[ "$status" -eq 0 ]
	run_vgrep -s c 10-15
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "10" ]]
}

@test "Show tree" {
	run_vgrep f
	[ "$status" -eq 0 ]
	run_vgrep -s t
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "Matches" ]]
	[[ ${lines[0]} =~ "Directory" ]]
}

@test "Show files" {
	run_vgrep f
	[ "$status" -eq 0 ]
	run_vgrep -sf
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "Matches" ]]
	[[ ${lines[0]} =~ "File" ]]
}


@test "Show delete" {
	run_vgrep f
	[ "$status" -eq 0 ]
	line2="${lines[2]}"
	run_vgrep -s d0
	[ "$status" -eq 0 ]
	run_vgrep -s d0,1-10,21,22,50
	[ "$status" -eq 0 ]
}

@test "Show ?" {
	run_vgrep f
	[ "$status" -eq 0 ]
	run_vgrep -s?
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "vgrep command help: command[context lines] [selectors]" ]]
	[[ ${lines[1]} =~ "selectors: '3' (single), '1,2,6' (multi), '1-8' (range)" ]]
	run_vgrep -s ?
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "vgrep command help: command[context lines] [selectors]" ]]
}
