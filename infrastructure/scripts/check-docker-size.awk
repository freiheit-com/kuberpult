#!/usr/bin/awk -f
{
	ACTUAL = $1
	ACTUAL_IN_MB = ACTUAL / 1024 / 1024
	TARGET_IN_MB = $2
	DISCREPANCY_PERCENT = $3
	DISCREPANCY_LIMIT_UPPER_MB = TARGET_IN_MB * (1 + DISCREPANCY_PERCENT / 100)
	DISCREPANCY_LIMIT_LOWER_MB = TARGET_IN_MB * (1 - DISCREPANCY_PERCENT / 100)
	if(ACTUAL_IN_MB > DISCREPANCY_LIMIT_UPPER_MB)
	{
		printf "Image too big. Actual size is %.2f MB, but target is %.2f MB (%.2f MB with discrepancy).", ACTUAL_IN_MB, TARGET_IN_MB, DISCREPANCY_LIMIT_UPPER_MB
		exit 11
	}
	if(ACTUAL_IN_MB < DISCREPANCY_LIMIT_LOWER_MB)
	{
		printf "Target too high. Actual size is %.2f MB, but target is %.2f MB (%.2f MB with discrepancy).", ACTUAL_IN_MB, TARGET_IN_MB, DISCREPANCY_LIMIT_LOWER_MB
		exit 12
	}
	printf "Image meets target (actual: %.2f MB, target: %.2f-%.2f MB", ACTUAL_IN_MB, DISCREPANCY_LIMIT_LOWER_MB, DISCREPANCY_LIMIT_UPPER_MB
}
