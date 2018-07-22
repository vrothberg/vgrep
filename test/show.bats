#!/usr/bin/env bats -t

@test "Show print" {
	run ./build/vgrep f
	[ "$status" -eq 0 ]
	run ./build/vgrep -s "p 10"
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "Index" ]]
	[[ ${lines[0]} =~ "File" ]]
	[[ ${lines[0]} =~ "Line" ]]
	[[ ${lines[0]} =~ "Content" ]]
	[[ ${lines[1]} =~ "10" ]]
}

@test "Show context with selectors" {
	run ./build/vgrep f
	[ "$status" -eq 0 ]
	run ./build/vgrep -s "c 10-15"
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "10" ]]
}

@test "Show tree" {
	run ./build/vgrep f
	[ "$status" -eq 0 ]
	run ./build/vgrep -st
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "Matches" ]]
	[[ ${lines[0]} =~ "Directory" ]]
}

@test "Show files" {
	run ./build/vgrep f
	[ "$status" -eq 0 ]
	run ./build/vgrep -sf
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "Matches" ]]
	[[ ${lines[0]} =~ "File" ]]
}


@test "Show delete" {
	run ./build/vgrep f
	[ "$status" -eq 0 ]
	line2="${lines[2]}"
	run ./build/vgrep -s "d0"
	[ "$status" -eq 0 ]
}

@test "Show ?" {
	run ./build/vgrep f
	[ "$status" -eq 0 ]
	run ./build/vgrep -s?
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ "vgrep command help: command[context lines] [selectors]" ]]
	[[ ${lines[1]} =~ "selectors: '3' (single), '1,2,6' (multi), '1-8' (range)" ]]
}
