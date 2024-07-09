COVERAGE_PATH=${COVERAGE_PATH:-`pwd`/.coverage}
VGREP=${VGREP:-`pwd`/build/vgrep}

function random_string() {
    local length=${1:-10}

    head /dev/urandom | tr -dc a-zA-Z0-9 | head -c$length
}

function run_vgrep() {
	local args=""
	if [[ -n "$COVERAGE" ]]; then
		args="-test.coverprofile=coverprofile.integration.$(random_string 20) COVERAGE"
		export GOCOVERDIR=${COVERAGE_PATH}
	fi
	run $VGREP $args "$@"
	if [ "$status" -ne 0 ]; then
		echo "-------------"
		echo "CLI: $VGREP $args $*"
		echo "OUT: $output"
		echo "-------------"
	fi
}

function is_root() {
    [ "$(id -u)" -eq 0 ]
}
