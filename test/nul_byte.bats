#!/usr/bin/env bats -t

load helpers

# Grep for a pattern in a line that contains NUL-bytes, and make sure we print
# the entire line.
#
# The grep tools consider files with NUL-bytes as binary, so we need to pass
# the '-a' option to tell them to process as a text file.

NUL_BYTE_FILE=test/search_files/nul_bytes.txt

@test "Search file with NUL-bytes (--no-git)" {
	run_vgrep --no-git -a NUL_BYTES $NUL_BYTE_FILE
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "END_OF_LINE" ]]
}

@test "Search file with NUL-bytes (--no-ripgrep)" {
	run_vgrep --no-git -a NUL_BYTES $NUL_BYTE_FILE
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "END_OF_LINE" ]]
}

@test "Search file with NUL-bytes (--no-git --no-ripgrep)" {
	run_vgrep --no-git --no-ripgrep -a NUL_BYTES $NUL_BYTE_FILE
	[ "$status" -eq 0 ]
	[[ ${lines[1]} =~ "END_OF_LINE" ]]
}
