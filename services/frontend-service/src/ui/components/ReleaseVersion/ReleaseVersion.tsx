/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com*/

import { Release } from '../../../api/api';

export type ReleaseVersionProps = { release: Pick<Release, 'sourceCommitId' | 'displayVersion'> };

export const ReleaseVersion: React.FC<ReleaseVersionProps> = ({ release }) => (
    <span className="commit-id" title={release.sourceCommitId}>
        {release.displayVersion} - {release.sourceCommitId}
    </span>
);
