#!/usr/bin/env bats -t

load helpers

@test "EDITOR not set" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	unset EDITOR
	run_vgrep -s 0
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ .*/vim ]]
	args=(${lines[1]})
	[[ ${args[0]} =~ .*/editor.bats ]]
	[[ ${args[1]} == +5 ]]	# first occurence of 'test' is on line 5
}


@test "EDITORs that require line number before path" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	for cmd in emacs{,client} nano; do
	    export EDITOR=$cmd
	    run_vgrep -s 0
	    [ "$status" -eq 0 ]
	    [[ ${lines[0]} =~ .*/$cmd ]]
	    args=(${lines[1]})
	    [[ ${args[0]} == +5 ]]
	    [[ ${args[1]} =~ .*/editor.bats ]]
	done
}

@test "EDITOR command with options" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	export EDITOR="emacs -nw"
	run_vgrep -s 0
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ .*/emacs ]]
	args=(${lines[1]})
	[[ ${args[0]} == -nw ]]
	[[ ${args[1]} == +5 ]]
	[[ ${args[2]} =~ .*/editor.bats ]]
}

@test "EDITORLINEFLAG" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	export EDITOR=kate
	export EDITORLINEFLAG=-l
	run_vgrep -s 0
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ .*/kate ]]
	args=(${lines[1]})
	[[ ${args[0]} =~ .*/editor.bats ]]
	[[ ${args[1]} == -l5 ]]
}

@test "EDITORLINEFLAGREVERSED" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	unset EDITOR
	export EDITORLINEFLAGREVERSED=1
	run_vgrep -s 0
	[ "$status" -eq 0 ]
	args=(${lines[1]})
	[[ ${args[0]} == +5 ]]
	[[ ${args[1]} =~ .*/editor.bats ]]
}
