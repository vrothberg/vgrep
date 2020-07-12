#!/usr/bin/env bats -t

@test "Interactive mode and delete with selectors" {
	./build/vgrep peanut test/search_files/foobar.txt > /dev/null
	run ./build/vgrep --show d 1,2-3,5,7-9,8-10 --interactive --no-header << EOF
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
