#!/usr/bin/env bash
set -euo pipefail

if (( $# < 3 )); then
    echo "usage: $0 <coverage_out_file> <TEST_COVERAGE_THRESHOLD> <PRODUCT_NAME>" >&2
    exit 2
fi

COVERAGE_OUT=$1
TEST_COVERAGE_THRESHOLD=$2
PRODUCT_NAME=$3
DISCREPANCY=5.0

# skip coverage test if threshold is set to 0.0 or below
if [ "$(awk "BEGIN { print ($TEST_COVERAGE_THRESHOLD < 0.0)? 1 : 0 }")" = 1 ]; then
    echo "Skipping coverage check";
    exit 0;
fi

# remove generated code for coverage calculation
grep -v -i -E 'pb.go|zz_generated.deepcopy.go' "$COVERAGE_OUT" > cover-filtered.out

LINE_COV=$(go tool cover "-func=cover-filtered.out" | grep total | sed 's/[^0-9\.]*//g' | tr -d '\n')
rm cover-filtered.out

if [ "$(awk "BEGIN { print ($TEST_COVERAGE_THRESHOLD > $LINE_COV)? 1 : 0 }")" = 1 ]; then
    echo "Coverage too low $LINE_COV<$TEST_COVERAGE_THRESHOLD in $PRODUCT_NAME";
    exit 1;
else
    if [ "$(awk "BEGIN { print ($LINE_COV > $TEST_COVERAGE_THRESHOLD + $DISCREPANCY)? 1 : 0 }")" = 1 ]; then
        echo "The current coverage $LINE_COV is more than $DISCREPANCY percent greater than the threshold $TEST_COVERAGE_THRESHOLD. Please adjust the threshold of $PRODUCT_NAME accordingly!"
        exit 1;
    fi

    echo "Coverage threshold met $LINE_COV>=$TEST_COVERAGE_THRESHOLD in $PRODUCT_NAME";
fi
