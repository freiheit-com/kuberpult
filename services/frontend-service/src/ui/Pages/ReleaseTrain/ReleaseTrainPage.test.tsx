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

import { MemoryRouter } from 'react-router-dom';
import { ReleaseTrainPrognosis } from '../../components/ReleaseTrainPrognosis/ReleaseTrainPrognosis';
import { render } from '@testing-library/react';
import { GetReleaseTrainPrognosisResponse, ReleaseTrainEnvSkipCause } from '../../../api/api';

test('ReleaseTrain component does not render anything if the response is undefined', () => {
    const { container } = render(
        <MemoryRouter>
            <ReleaseTrainPrognosis releaseTrainPrognosis={undefined} />
        </MemoryRouter>
    );
    expect(container.textContent).toContain('Backend returned empty response');
});

test('ReleaseTrain component renders release train prognosis when the response is valid', () => {
    type Table = {
        head: string[];
        // NOTE: newlines, if there are any, will effectively be removed, since they will be checked using .toHaveTextContent
        body: string[][];
    };

    type EnvReleaseTrainPrognosisModel = {
        id: string;
        headerText: string;
        content: string | Table;
    };

    type TestCase = {
        releaseTrainPrognosis: GetReleaseTrainPrognosisResponse;
        expectedPageContent: EnvReleaseTrainPrognosisModel[];
    };

    const testCases: TestCase[] = [
        {
            releaseTrainPrognosis: {
                envsPrognoses: {
                    'env-1': {
                        outcome: {
                            $case: 'skipCause',
                            skipCause: ReleaseTrainEnvSkipCause.ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
                        },
                    },
                    // 'env-2': {
                    //     outcome: {
                    //         $case: 'skipCause',
                    //         skipCause: ReleaseTrainEnvSkipCause.ENV_HAS_NO_UPSTREAM,
                    //     },
                    // },
                },
            },
            expectedPageContent: [
                {
                    id: 'env-1',
                    headerText: 'potato',
                    content: 'potahto',
                },
            ],
        },
    ];

    for (const testCase of testCases) {
        const { container } = render(
            <MemoryRouter>
                <ReleaseTrainPrognosis releaseTrainPrognosis={testCase.releaseTrainPrognosis} />
            </MemoryRouter>
        );

        // 
    }
});
