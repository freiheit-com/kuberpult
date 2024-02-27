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
import { act, render } from '@testing-library/react';
import { DotsMenu, DotsMenuProps } from './DotsMenu';
import { elementQuerySelectorSafe } from '../../../setupTests';

type TestData = {
    name: string;
    input: DotsMenuProps;
    expectedNumItems: number;
};

const mySpy = jest.fn();

// TODO: SRX-UJ4PMT tests for close on click outside

const data: TestData[] = [
    {
        name: 'renders empty list',
        input: { buttons: [] },
        expectedNumItems: 0,
    },
    {
        name: 'renders one button',
        input: {
            buttons: [
                {
                    label: 'test label',
                    onClick: mySpy,
                },
            ],
        },
        expectedNumItems: 1,
    },
    {
        name: 'renders three button',
        input: {
            buttons: [
                {
                    label: 'test label A',
                    onClick: mySpy,
                },
                {
                    label: 'test label B',
                    onClick: mySpy,
                },
                {
                    label: 'test label C',
                    onClick: mySpy,
                },
            ],
        },
        expectedNumItems: 3,
    },
];

describe('DotsMenu Rendering', () => {
    const getNode = (overrides: DotsMenuProps) => <DotsMenu {...overrides} />;
    const getWrapper = (overrides: DotsMenuProps) => render(getNode(overrides));
    describe.each(data)('DotsMenu Test', (testcase) => {
        it(testcase.name, () => {
            mySpy.mockReset();
            expect(mySpy).toHaveBeenCalledTimes(0);

            const { container } = getWrapper(testcase.input);

            expect(document.querySelectorAll('.dots-menu-hidden .mdc-button--unelevated').length).toEqual(1);
            const result = elementQuerySelectorSafe(container, '.dots-menu-hidden .mdc-button--unelevated');
            act(() => {
                result.click();
            });
            expect(document.querySelectorAll('.dots-menu-hidden .mdc-button--unelevated').length).toEqual(0);
            expect(document.querySelectorAll('li.item').length).toEqual(testcase.expectedNumItems);

            if (testcase.expectedNumItems > 0) {
                expect(mySpy).toHaveBeenCalledTimes(0);
                const result = elementQuerySelectorSafe(container, 'li .mdc-button--unelevated');
                act(() => {
                    result.click();
                });
                expect(mySpy).toHaveBeenCalledTimes(1);
            }
        });
    });
});
