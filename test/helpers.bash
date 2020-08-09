
VGREP=${VGREP:-`pwd`/build/vgrep}

function run_vgrep() {
	run $VGREP "$@"
}
