#!/usr/bin/env bats -t

load helpers

FILE=test/search_files/foobar.txt

@test "Selectors: discrete selection" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p 0,1,2,3,4
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 5 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: range" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p 0-4
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 5 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: mix" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p 0-4,5,8
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 7 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: mix, with spaces" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p 0 - 4, 5, 8
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 7 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: context with print (no effect)" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p3 0-4
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 5 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: context with range" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --show c2 2-4
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 18 ]]
	[[ ${lines[15]} =~ "four" ]]
}

@test "Selectors: empty selection" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 11 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: all" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p all
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 11 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: index out of range" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p 999
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 1 ]]
	[[ ${lines[@]} =~ "out of range" ]]
}

@test "Selectors: range out of range" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --no-header --show p 0-999
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 1 ]]
	[[ ${lines[@]} =~ "out of range" ]]
}
