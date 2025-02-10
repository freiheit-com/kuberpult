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
import React from 'react';
import { render } from '@testing-library/react';
import { ProgressBar, ProgressBarProps } from './ProgressBar';

describe('ProgressBar', () => {
    const defaultProps: ProgressBarProps = {
        max: 100,
        value: 0,
    };

    const getNode = (props: Partial<ProgressBarProps>): JSX.Element => (
        <ProgressBar {...Object.assign({}, defaultProps, props)} />
    );

    const getWrapper = (props: Partial<ProgressBarProps>) => render(getNode(props));

    type TestData = {
        name: string;
        props: Partial<ProgressBarProps>;
        expectedLabel: string;
    };

    const data: TestData[] = [
        {
            name: 'progress bar with 0 as the value',
            props: {},
            expectedLabel: '0%',
        },
        {
            name: 'progress bar with 50 as the value',
            props: { value: 50 },
            expectedLabel: '50%',
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            const { container } = getWrapper(testcase.props);

            const span = container.querySelector('span');
            expect(span === null).toBe(false);
            expect(span?.textContent).toEqual(testcase.expectedLabel);

            const progress = container.querySelector('progress');
            expect(progress === null).toBe(false);
            expect(progress?.value).toEqual(testcase.props.value ?? defaultProps.value);
            expect(progress?.max).toEqual(testcase.props.max ?? defaultProps.max);
        });
    });
});
