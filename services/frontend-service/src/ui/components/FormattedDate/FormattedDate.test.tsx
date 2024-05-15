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
import { render } from '@testing-library/react';
import { FormattedDate } from './FormattedDate';

describe('Relative Date Calculation', () => {
    // the test release date ===  18/06/2001 is constant across this test
    const testReleaseDate = new Date(2001, 5, 8);

    const data = [
        {
            name: 'now',
            systemTime: new Date(2001, 5, 8, 0, 0),
            expected: 'just now',
        },
        {
            name: '1 minute ago',
            systemTime: new Date(2001, 5, 8, 0, 1),
            expected: '1 minute ago',
        },
        {
            name: '5 minutes ago',
            systemTime: new Date(2001, 5, 8, 0, 5),
            expected: '5 minutes ago',
        },
        {
            name: '1 hour ago',
            systemTime: new Date(2001, 5, 8, 1, 1),
            expected: '1 hour ago',
        },
        {
            name: '5 hours ago',
            systemTime: new Date(2001, 5, 8, 5, 1),
            expected: '5 hours ago',
        },
        {
            name: 'little over 1 day ago',
            systemTime: new Date(2001, 5, 9, 1, 1),
            expected: '1 day ago',
        },
        {
            name: '3 days ago',
            systemTime: new Date(2001, 5, 11, 1, 1),
            expected: '3 days ago',
        },
        {
            name: '3 months ago',
            systemTime: new Date(2001, 8, 8, 5, 1),
            expected: '3 months ago',
        },
    ];

    describe.each(data)('calculates the right date and time', (testcase) => {
        it(testcase.name, () => {
            // given
            jest.useFakeTimers(); // fake time is now "0"
            jest.setSystemTime(testcase.systemTime.valueOf()); // time is now at the exact moment when release was created
            const { container } = render(<FormattedDate createdAt={testReleaseDate} />);

            // then
            expect(container.textContent).toContain(testcase.expected);

            // finally
            jest.runOnlyPendingTimers();
            jest.useRealTimers();
        });
    });
});
