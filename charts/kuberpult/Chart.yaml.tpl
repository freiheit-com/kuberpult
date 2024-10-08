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
apiVersion: v2
name: kuberpult
description: freiheit.com contiuous delivery

# A chart can be either an 'application' or a 'library' chart.
#
# Application charts are a collection of templates that can be packaged into versioned archives
# to be deployed.
#
# Library charts provide useful utilities or functions for the chart developer. They're included as
# a dependency of application charts to inject those utilities and functions into the rendering
# pipeline. Library charts do not define any templates and therefore cannot be deployed.
type: application

# This is the chart version. This version number should be incremented each time you make changes
# to the chart and its templates, including the app version.
# Versions are expected to follow Semantic Versioning (https://semver.org/)
version: "${CHARTS_VERSION}"

# This is the version number of the application being deployed. This version number should be
# incremented each time you make changes to the application. Versions are not expected to
# follow Semantic Versioning. They should reflect the version the application is using.
# It is recommended to use it with quotes.
appVersion: "${VERSION}"

# This is the DEX helm chart which will only be installed if `auth.dexAuth.installDex.enabled` is true.
# Dex is an identity service that uses OpenID Connect to drive authentication through other
# identity providers.
# For more information please check: https://github.com/dexidp/dex
dependencies:
- name: dex
  condition: auth.dexAuth.installDex
  version: "0.17.1"
  repository: https://charts.dexidp.io

maintainers:
 - name: hannesg
 - name: sven-urbanski-freiheit-com
