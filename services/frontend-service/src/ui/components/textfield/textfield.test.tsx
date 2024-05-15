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
import { Textfield } from './textfield';
import { render } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { getElementsByClassNameSafe } from '../../../setupTests';

describe('Textfield', () => {
    it('renders correctly using Snapshot', () => {
        // given
        const { container } = render(
            <MemoryRouter>
                <Textfield placeholder="Floating label" />
            </MemoryRouter>
        );
        // when & then
        expect(container.firstChild).toMatchSnapshot();
    });

    test('renders correctly with leading icon', () => {
        // given
        const { container } = render(
            <MemoryRouter>
                <Textfield leadingIcon="search" />
            </MemoryRouter>
        );
        // when & then
        expect(container.querySelectorAll('div')[0]?.className).toEqual(
            'mdc-text-field mdc-text-field--outlined mdc-text-field--no-label mdc-text-field--with-leading-icon'
        );
        expect(container.querySelector('i')).toMatchInlineSnapshot(`
    <i
      class="material-icons mdc-text-field__icon mdc-text-field__icon--leading"
      tabindex="0"
    >
      search
    </i>
  `);
    });
});

describe('Verify textfield content', () => {
    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return (
            <MemoryRouter>
                <Textfield {...defaultProps} {...overrides} />;
            </MemoryRouter>
        );
    };

    const getWrapper = (overrides?: { placeholder: string; value: string; className: string }, entries?: string[]) =>
        render(getNode(overrides));

    it(`Renders a navigation item base`, () => {
        // when
        const { container } = getWrapper();
        // then
        expect(container.firstChild);
    });

    interface dataT {
        name: string;
        className: string;
        placeholder: string;
        value: string;
        expect: (container: HTMLElement) => void;
    }

    const data: dataT[] = [
        {
            name: 'Empty textfield',
            className: 'top-app-bar-search-field',
            placeholder: 'Search',
            value: '',
            expect: (container) =>
                expect(container.getElementsByClassName('mdc-text-field__input')[0]).toHaveTextContent(''),
        },
        {
            name: 'Textfield with content',
            className: 'top-app-bar-search-field',
            placeholder: 'Search',
            value: 'test-search',
            expect: (container) => {
                const input = getElementsByClassNameSafe(container, 'mdc-text-field__input')[0];
                input.nodeValue = 'test-search';
                return expect(container.getElementsByClassName('mdc-text-field__input')[0]).toHaveDisplayValue(
                    'test-search'
                );
            },
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            const { className, placeholder, value } = testcase;
            // when
            const { container } = getWrapper({ placeholder, className, value });
            // then
            testcase.expect(container);
        });
    });
});
