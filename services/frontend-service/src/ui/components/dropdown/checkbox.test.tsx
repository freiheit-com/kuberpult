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

import { act, render } from '@testing-library/react';
import { Checkbox, CheckboxProps } from './checkbox';
import { documentQuerySelectorSafe } from '../../../setupTests';

const getNode = (props: CheckboxProps): JSX.Element => <Checkbox {...props} />;

const getWrapper = (input: CheckboxProps) => render(getNode(input));

describe('Checkbox', () => {
    interface dataT {
        name: string;
        input: CheckboxProps;
        expectedText: string;
    }
    const mySubmitSpy = jest.fn();

    const data: dataT[] = [
        {
            name: 'Test onClick',
            input: { id: 'id1', enabled: true, label: 'alpha label', onClick: mySubmitSpy },
            expectedText: 'alpha label',
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            // given
            mySubmitSpy.mockReset();
            // when
            getWrapper(testcase.input);
            // then
            const result = documentQuerySelectorSafe('.id-' + testcase.input.id);
            expect(mySubmitSpy).toHaveBeenCalledTimes(0);
            act(() => {
                result.click();
            });
            expect(mySubmitSpy).toHaveBeenCalledTimes(1);
            expect(mySubmitSpy).toHaveBeenCalledWith(testcase.input.id);

            expect(document.querySelectorAll('.checkbox-wrapper').length).toEqual(1);
            expect(document.querySelectorAll('.checkbox-wrapper')[0]).toHaveTextContent(testcase.expectedText);
        });
    });
});
