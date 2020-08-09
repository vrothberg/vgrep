#!/usr/bin/env bats -t

load helpers

FILE=test/search_files/foobar.txt

@test "Interactive mode and delete with selectors" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --show d 1,2-3,5,7-9,8-10 --interactive --no-header << EOF
p
q
EOF
	[ "$status" -eq 0 ]
	# We expect 3 results, but there is also a prompt line in the output
	[[ ${#lines[*]} -eq 4 ]]
	[[ ${lines[0]} =~ "zero" ]]
	[[ ${lines[1]} =~ "four" ]]
	[[ ${lines[2]} =~ "six" ]]
}

@test "Interactive mode and keep with selectors" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --show k 0,4,6 --interactive --no-header << EOF
p
q
EOF
	[ "$status" -eq 0 ]
	# We expect 3 results, but there is also a prompt line in the output
	[[ ${#lines[*]} -eq 4 ]]
	[[ ${lines[0]} =~ "zero" ]]
	[[ ${lines[1]} =~ "four" ]]
	[[ ${lines[2]} =~ "six" ]]
}

@test "Interactive mode and refine with regexp" {
	run_vgrep peanut $FILE > /dev/null
	run_vgrep --show r '(zero|f[^i].* p|six)' --interactive --no-header << EOF
p
q
EOF
	[ "$status" -eq 0 ]
	# We expect 3 results, but there is also a prompt line in the output
	[[ ${#lines[*]} -eq 4 ]]
	[[ ${lines[0]} =~ "zero" ]]
	[[ ${lines[1]} =~ "four" ]]
	[[ ${lines[2]} =~ "six" ]]
}

@test "Interactive mode and new grep search" {
	run_vgrep my_pattern > /dev/null
	run_vgrep --show "g -w peanut $FILE" \
		--interactive --no-header << EOF
q
EOF
	[ "$status" -eq 0 ]
	# We expect 2 results, but there is also a prompt line in the output
	[[ ${#lines[*]} -eq 3 ]]
	[[ ${lines[0]} =~ "zero" ]]
	[[ ${lines[1]} =~ "one" ]]
}
