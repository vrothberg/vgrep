#!/usr/bin/env bats -t

load helpers

@test "EDITOR not set" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	unset EDITOR
	run_vgrep -s 0
	[ "$status" -eq 0 ]
	[[ ${lines[0]/editor: /} =~ .*/vim ]]
	args=(${lines[1]/args: /})
	[[ ${args[0]} =~ .*/editor.bats ]]
	[[ ${args[1]} == +5 ]]	# first occurence of 'test' is on line 5
}


@test "EDITORs that require line number before path" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	for cmd in emacs{,client} nano; do
	    EDITOR=$cmd
	    run_vgrep -s 0
	    [ "$status" -eq 0 ]
	    [[ ${lines[0]/editor: /} =~ .*/$cmd ]]
	    args=(${lines[1]/args: /})
	    [[ ${args[0]} == +5 ]]
	    [[ ${args[1]} =~ .*/editor.bats ]]
	done
}

@test "EDITOR command with options" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	EDITOR="emacs -nw"
	run_vgrep -s 0
	[ "$status" -eq 0 ]
	[[ ${lines[0]/editor: /} =~ .*/emacs ]]
	args=(${lines[1]/args: /})
	[[ ${args[0]} == -nw ]]
	[[ ${args[1]} == +5 ]]
	[[ ${args[2]} =~ .*/editor.bats ]]
}

@test "EDITORLINEFLAG" {
	run_vgrep test test/editor.bats
	[ "$status" -eq 0 ]
	EDITOR=kate
	export EDITORLINEFLAG=-l
	run_vgrep -s 0
	[ "$status" -eq 0 ]
	[[ ${lines[0]/editor: /} =~ .*/kate ]]
	args=(${lines[1]/args: /})
	[[ ${args[0]} =~ .*/editor.bats ]]
	[[ ${args[1]} == -l5 ]]
}
