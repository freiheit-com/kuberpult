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

Copyright freiheit.com*/

import { GetReleaseTrainPrognosisResponse } from '../../../api/api';
import { TopAppBar } from '../TopAppBar/TopAppBar';

type ReleaseTrainProps = {
    releaseTrainPrognosis: GetReleaseTrainPrognosisResponse | undefined;
};

export const ReleaseTrain: React.FC<ReleaseTrainProps> = (props) => {
    const releaseTrainPrognosis = props.releaseTrainPrognosis;

    if (releaseTrainPrognosis === undefined) {
        return (
            <div>
                <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                <main className="main-content commit-page">Backend returned empty response</main>
            </div>
        );
    }

    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            <main className="main-content commit-page">{JSON.stringify(releaseTrainPrognosis)}</main>
        </div>
    );
};
