#!/usr/bin/env bats -t

FILE=test/search_files/foobar.txt

@test "Selectors: discrete selection" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p 0,1,2,3,4
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 5 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: range" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p 0-4
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 5 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: mix" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p 0-4,5,8
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 7 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: mix, with spaces" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p 0 - 4, 5, 8
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 7 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: context with print (no effect)" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p3 0-4
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 5 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: context with range" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --show c2 2-4
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 18 ]]
	[[ ${lines[15]} =~ "four" ]]
}

@test "Selectors: empty selection" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 11 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: all" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p all
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 11 ]]
	[[ ${lines[4]} =~ "four" ]]
}

@test "Selectors: index out of range" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p 999
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 1 ]]
	[[ ${lines[@]} =~ "out of range" ]]
}

@test "Selectors: range out of range" {
	./build/vgrep peanut $FILE > /dev/null
	run ./build/vgrep --no-header --show p 0-999
	[ "$status" -eq 0 ]
	[[ ${#lines[*]} -eq 1 ]]
	[[ ${lines[@]} =~ "out of range" ]]
}
