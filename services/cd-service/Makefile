# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright freiheit.com

MIN_COVERAGE=63.4
MIN_COVERAGE_TRAIN=80.5
MIN_COVERAGE_DEPLOY=83.8
SKIP_BUILDER=1
include ../../infrastructure/make/go/include.mk

test: test-with-coverage
test-with-coverage: unit-test
	# unit-test already has a coverage check, but only for the whole service
	# here we add coverage checks with higher thresholds, because these are our core processes.
	# coverage files need to begin with 'mode:set' in the first line, so we add it:
	(echo 'mode: set'; (cat coverage.out | grep '/releasetrain.go')) > coverage.train.out
	(echo 'mode: set'; (cat coverage.out | grep '/deployment.go')) > coverage.deploy.out
	$(ROOT_DIR)/infrastructure/coverage/check-coverage-go.sh coverage.train.out $(MIN_COVERAGE_TRAIN) releasetrain.go
	$(ROOT_DIR)/infrastructure/coverage/check-coverage-go.sh coverage.deploy.out $(MIN_COVERAGE_DEPLOY) deployment.go