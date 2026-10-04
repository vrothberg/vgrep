#!/usr/bin/env bats -t

load helpers

@test "NO_COLOR suppresses ANSI codes in output" {
	NO_COLOR=1 run_vgrep f
	[ "$status" -eq 0 ]
	# Make sure no ANSI escape sequences are present
	! [[ ${output} =~ $'\033' ]]
}

@test "NO_COLOR suppresses ANSI codes with --no-header" {
	NO_COLOR=1 run_vgrep --no-header f
	[ "$status" -eq 0 ]
	! [[ ${output} =~ $'\033' ]]
}

@test "NO_COLOR passes --color=never to ripgrep" {
	NO_COLOR=1 run_vgrep -d some_pattern 2>&1
	[[ ${lines[@]} =~ "--color=never" ]]
	! [[ ${lines[@]} =~ "--color=always" ]]
}

@test "NO_COLOR passes --color=never to git grep" {
	NO_COLOR=1 run_vgrep -d --no-ripgrep some_pattern 2>&1
	[[ ${lines[@]} =~ "--color=never" ]]
	! [[ ${lines[@]} =~ "color.grep.match" ]]
}

@test "NO_COLOR passes --color=never to classic grep" {
	NO_COLOR=1 run_vgrep -d --no-ripgrep --no-git some_pattern 2>&1
	[[ ${lines[@]} =~ "--color=never" ]]
	! [[ ${lines[@]} =~ "--color=always" ]]
}

@test "Output has ANSI codes without NO_COLOR" {
	run_vgrep f
	[ "$status" -eq 0 ]
	[[ ${output} =~ $'\033' ]]
}

@test "NO_COLOR='' does not suppress colors" {
	NO_COLOR='' run_vgrep f
	[ "$status" -eq 0 ]
	[[ ${output} =~ $'\033' ]]
}
